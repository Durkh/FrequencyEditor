package image

import (
	"errors"
	"github.com/Durkh/FrequencyEditor/Freq"
	"github.com/Durkh/FrequencyEditor/culling"
	"golang.org/x/image/tiff"
	im "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Image struct {
	Image im.Image
	Name  string
}

func (i *Image) Open(path string) (err error) {

	var (
		f *os.File
	)

	if f, err = os.Open(path); err != nil {
		return err
	}

	defer f.Close()

	switch filepath.Ext(path) {
	case ".tif":
		fallthrough
	case ".tiff":
		i.Image, err = tiff.Decode(f)
	case ".png":
		i.Image, err = png.Decode(f)
	case ".jpg":
		fallthrough
	case ".jpeg":
		i.Image, err = jpeg.Decode(f)
	default:
		return errors.New("unknown format")
	}

	old := i.Image
	i.Image = im.NewGray(i.Image.Bounds())

	iterate(i.Image.Bounds(), func(x int, y int) {
		i.Image.(*im.Gray).Set(x, y, old.At(x, y))
	})

	i.Name = strings.Split(filepath.Base(path), ".")[0]

	return
}

func (i *Image) DCT(args map[string]interface{}) *Freq.Frequencies {

	var (
		//histogram min/max
		min, max    = math.MaxFloat64, -math.MaxFloat64
		minMaxMutex sync.Mutex
		minMaxFunc  = func(val float64) {}
		noDC        bool
	)

	//variable changes for removal of DC signal and histogram expansion
	if _, noDC = args["histogram"]; noDC {
		minMaxFunc = func(val float64) {
			minMaxMutex.Lock()

			if val < min {
				min = val
			}
			if val > max {
				max = val
			}

			minMaxMutex.Unlock()
		}
	}

	var (
		bounds = i.Image.Bounds()

		partial = Freq.NewFreq(bounds.Max.Y, bounds.Max.X)
		res     = Freq.NewFreq(bounds.Max.Y, bounds.Max.X, i.Name+"_DCT_")

		cf *int
	)

	if _, ok := args["cutFrequency"]; ok {
		cf = args["cutFrequency"].(*int)
	}

	//horizontal DCT
	//in this case: outer = Y; inner = X
	iterate(bounds, func(inner int, outer int) {

		assignFunc := func(val *float64) {
			//rawVal[Y][X] = val
			partial.Data2D[outer][inner] = *val
		}

		accessFunc := func(index *int) float64 {
			//image.at(X<<ranged>>, Y<<fixed>>)
			return float64(i.Image.At(*index, outer).(color.Gray).Y)
		}

		DCT1d(inner, bounds.Max.X, math.Sqrt(2/float64(bounds.Max.X)), limits{low: bounds.Min.X, high: bounds.Max.X},
			accessFunc, assignFunc, func(val float64) {})
	})

	//vertical DCT
	//inverting image iteration
	b := im.Rect(
		bounds.Min.Y, bounds.Min.X,
		bounds.Max.Y, bounds.Max.X,
	)
	//in this case: outer = X; inner = Y
	iterate(b, func(inner int, outer int) {

		assignFunc := func(val *float64) {
			//to show the coefficients we need to get their norm
			if noDC {
				*val = math.Abs(*val)
			}
			//rawVal[Y][X] = val
			res.Data2D[inner][outer] = *val
		}

		accessFunc := func(index *int) float64 {
			//rawVal[Y<<ranged>>][X<<fixed>>]
			return partial.Data2D[*index][outer]
		}

		//don't get the DC value as maximum
		if outer == 0 && inner == 0 {
			DCT1d(inner, bounds.Max.Y, math.Sqrt(2/float64(bounds.Max.Y)), limits{low: bounds.Min.Y, high: bounds.Max.Y},
				accessFunc, assignFunc, func(val float64) {})
			return
		}

		DCT1d(inner, bounds.Max.Y, math.Sqrt(2/float64(bounds.Max.Y)), limits{low: bounds.Min.Y, high: bounds.Max.Y},
			accessFunc, assignFunc, minMaxFunc)
	})

	//removal of frequencies data
	if cf != nil {
		culling.Cull(*cf, res.Data2D)

		res.Filename += "CF_[" + strconv.Itoa(*cf) + "]"
	}

	//histogram expansion
	if noDC {
		histExpansion(min, max, res.Data2D)
	}

	return res
}

func (i *Image) IDCT(freq *Freq.Frequencies) Freq.Wave {

	var (
		bounds = i.Image.Bounds()

		partial = Freq.NewFreq(bounds.Max.Y, bounds.Max.X)
	)

	//horizontal DCT
	//in this case: outer = Y; inner = X
	iterate(bounds, func(inner int, outer int) {

		assignFunc := func(val *float64) {
			//rawVal[Y][X] = val
			partial.Data2D[outer][inner] = *val
		}

		accessFunc := func(index *int) float64 {
			//rawVal[Y<<fixed>>][X<<ranged>>]
			return freq.Data2D[outer][*index]
		}

		IDCT1d(inner, bounds.Max.X, math.Sqrt(2/float64(bounds.Max.X)), limits{low: bounds.Min.X, high: bounds.Max.X},
			accessFunc, assignFunc)
	})

	//vertical DCT
	//inverting image iteration
	b := im.Rect(
		bounds.Min.Y, bounds.Min.X,
		bounds.Max.Y, bounds.Max.X,
	)
	//in this case: outer = X; inner = Y
	iterate(b, func(inner int, outer int) {

		assignFunc := func(val *float64) {
			*val = math.Round(*val)
			if *val < 0 {
				*val = 0
			}
			if *val > 0xff {
				*val = 0xff
			}

			//rawVal[Y][X] = val
			freq.Data2D[inner][outer] = *val
		}

		accessFunc := func(index *int) float64 {
			//rawVal[Y<<ranged>>][X<<fixed>>]
			return partial.Data2D[*index][outer]
		}

		IDCT1d(inner, bounds.Max.Y, math.Sqrt(2/float64(bounds.Max.Y)), limits{low: bounds.Min.Y, high: bounds.Max.Y},
			accessFunc, assignFunc)
	})

	i.Image = freq.ToGray()
	i.Name = freq.Filename

	return i
}

//DCT1d
// k = index
// Xk = color
// ft = DCT's First Term
func DCT1d(k, N int, ft float64, bounds limits, access func(*int) float64, assign func(*float64), minMaxFunc func(float64)) {

	var (
		pixConst         = math.Pi * float64(k) / float64(N)
		Ck       float64 = 1
		sum, res float64
	)

	if k == 0 {
		Ck = math.Sqrt(1.0 / 2.0)
	}

	//DCT sum
	for n := bounds.low; n < bounds.high; n++ {
		sum += access(&n) * math.Cos(pixConst*(float64(n)+.5))
	}

	//constant part
	res = ft * Ck * sum
	assign(&res)

	//get image min and max values
	minMaxFunc(math.Abs(res))
}

func IDCT1d(n, N int, ft float64, bounds limits, access func(*int) float64, assign func(*float64)) {

	var (
		pixConst = (math.Pi / float64(N)) * (float64(n) + .5)
		Ck       = math.Sqrt(1.0 / 2.0)
		sum, res float64
	)

	//DCT sum
	for k := bounds.low; k < bounds.high; k++ {
		sum += access(&k) * math.Cos(pixConst*float64(k)) * Ck
		Ck = 1
	}

	//constant part
	res = ft * sum
	assign(&res)
}

type limits struct {
	low, high int
}

func histExpansion(min, max float64, hist [][]float64) {

	var (
		wg      sync.WaitGroup
		channel = make(chan *float64, 64)

		assign = func(rawVal <-chan *float64) {
			defer wg.Done()

			var delta = max - min

			for val := range rawVal {
				*val = math.Round(((*val - min) * 255.0) / delta)
			}
		}
	)

	defer wg.Wait()
	defer close(channel)

	//worker pool
	wg.Add(runtime.NumCPU() * 2)
	for i := 0; i < runtime.NumCPU()*2; i++ {
		go assign(channel)
	}

	//assign
	for itrV := 0; itrV < len(hist); itrV++ {
		for itrH := 0; itrH < len(hist[itrV]); itrH++ {
			channel <- &hist[itrV][itrH]
		}
	}

}

func iterate(bounds im.Rectangle, closure func(int, int)) {

	var (
		wg sync.WaitGroup

		funcChannel = make(chan im.Point, 64)
		middleman   = func(pixels <-chan im.Point) {
			defer wg.Done()
			for pixel := range pixels {
				closure(pixel.X, pixel.Y)
			}
		}
	)

	//cleanup
	defer wg.Wait()
	defer close(funcChannel)

	//worker pool
	wg.Add(runtime.NumCPU() * 2)
	for i := 0; i < runtime.NumCPU()*2; i++ {
		go middleman(funcChannel)
	}

	//iteration
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {

			funcChannel <- im.Point{X: x, Y: y}
		}
	}
}

func SaveImage(im Image) error {

	var (
		suffix string
		err    error
	)

	f, err := os.Create(im.Name + suffix + "_" + strconv.Itoa(int(time.Now().Unix())) + ".png")
	if err != nil {
		return err
	}

	defer f.Close()

	err = png.Encode(f, im.Image)
	if err != nil {
		return err
	}

	return nil
}

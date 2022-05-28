package image

import (
	"errors"
	"github.com/Durkh/FrequencyEditor/Freq"
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

func (i *Image) DCT(args map[string]interface{}) Freq.Wave {

	var (
		rect = i.Image.Bounds()

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

		rect = im.Rect(1, 1, rect.Max.X, rect.Max.Y)
	}

	var (
		freqIM = Image{
			Image: im.NewGray(rect),
			Name:  i.Name + "_DCT",
		}

		bounds = freqIM.Image.Bounds()

		//DCT's constant terms
		ft float64
		N  int

		//raw DCT values, pre histogram expansion
		rawValues = make([][]float64, bounds.Max.Y)
		//n loop limits
		lim limits
	)

	for itr := range rawValues {
		rawValues[itr] = make([]float64, bounds.Max.X)
	}

	//horizontal DCT
	N = i.Image.Bounds().Max.X
	ft = math.Sqrt(2 / float64(N))
	lim = limits{low: i.Image.Bounds().Min.X, high: N}
	//in this case: outer = Y; inner = X
	iterate(i.Image.Bounds(), func(inner int, outer int) {

		assignFunc := func(val float64) {
			//rawVal[Y][X] = val
			rawValues[outer][inner] = val
		}

		accessFunc := func(index int) float64 {
			//image.at(X<<ranged>>, Y<<fixed>>)
			return float64(i.Image.At(index, outer).(color.Gray).Y)
		}

		DCT1d(inner, N, ft, lim, accessFunc, assignFunc, func(val float64) {})
	})

	//vertical DCT
	N = i.Image.Bounds().Max.Y
	ft = math.Sqrt(2 / float64(N))
	lim = limits{low: i.Image.Bounds().Min.Y, high: N}
	//inverting image iteration
	b := im.Rect(
		i.Image.Bounds().Min.Y, i.Image.Bounds().Min.X,
		i.Image.Bounds().Max.Y, i.Image.Bounds().Max.X,
	)
	//in this case: outer = X; inner = Y
	iterate(b, func(inner int, outer int) {

		assignFunc := func(val float64) {
			//rawVal[Y][X] = val
			rawValues[inner][outer] = math.Abs(val)
		}

		accessFunc := func(index int) float64 {
			//rawVal[Y<<ranged>>][X<<fixed>>]
			return rawValues[index][outer]
		}

		if outer == 0 && inner == 0 {
			DCT1d(inner, N, ft, lim, accessFunc, assignFunc, func(val float64) {})
			return
		}

		DCT1d(inner, N, ft, lim, accessFunc, assignFunc, minMaxFunc)
	})

	//histogram expansion
	if noDC {
		histExpansion(min, max, rawValues)
	}

	rawValues[0][0] = 0xff

	//set
	iterate(bounds, func(x int, y int) {
		freqIM.Image.(*im.Gray).Set(x, y, color.Gray{Y: uint8(rawValues[y][x])})
	})

	return &freqIM
}

//DCT1d
// k = index
// Xk = color
// ft = DCT's First Term
func DCT1d(k, N int, ft float64, bounds limits, access func(int) float64, assign, minMaxFunc func(float64)) {

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
		sum += access(n) * math.Cos(pixConst*(float64(n)+.5))
	}

	//constant part
	res = ft * Ck * sum
	assign(res)

	//get image min and max values
	minMaxFunc(math.Abs(res))
}

//TODO remove later
func (i Image) DCT2D(args map[string]interface{}) Freq.Wave {

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
		freqIM = Image{
			Image: i.Image,
			Name:  i.Name + "_DCT",
		}

		bounds = freqIM.Image.Bounds()

		//DCT's constant terms
		ft float64
		N  int

		//raw DCT values, pre histogram expansion
		rawValues = make([][]float64, bounds.Max.Y)
	)

	iterate(im.Rect(0, 0, len(rawValues), 1), func(x int, y int) {
		rawValues[x] = make([]float64, bounds.Max.X)
		for itr := i.Image.Bounds().Min.X; itr < i.Image.Bounds().Max.X; itr++ {
			rawValues[x][itr] = float64(i.Image.At(itr, x).(color.Gray).Y)
		}
	})

	N = i.Image.Bounds().Max.X
	ft = math.Sqrt(2 / float64(N))
	for itrY := i.Image.Bounds().Min.Y; itrY < i.Image.Bounds().Max.Y; itrY++ {
		for itrX := i.Image.Bounds().Min.X; itrX < i.Image.Bounds().Max.X; itrX++ {
			var (
				pixConst         = math.Pi * float64(itrX) / float64(N)
				Ck       float64 = 1
				sum      float64
			)

			if itrX == 0 {
				Ck = math.Sqrt(1. / 2.)
			}

			for n := i.Image.Bounds().Min.X; n < i.Image.Bounds().Max.X; n++ {
				sum += float64(i.Image.At(n, itrY).(color.Gray).Y) * math.Cos(pixConst*(float64(n)+.5))
			}

			rawValues[itrY][itrX] = ft * Ck * sum
		}
	}

	N = i.Image.Bounds().Max.Y
	ft = math.Sqrt(2 / float64(N))
	for itrX := i.Image.Bounds().Min.X; itrX < i.Image.Bounds().Max.X; itrX++ {
		for itrY := i.Image.Bounds().Min.Y; itrY < i.Image.Bounds().Max.Y; itrY++ {
			var (
				pixConst         = math.Pi * float64(itrY) / float64(N)
				Ck       float64 = 1
				sum      float64
			)

			if itrY == 0 {
				Ck = math.Sqrt(1. / 2.)
			}

			for n := i.Image.Bounds().Min.Y; n < i.Image.Bounds().Max.Y; n++ {
				sum += rawValues[n][itrX] * math.Cos(pixConst*(float64(n)+.5))
			}

			rawValues[itrY][itrX] = math.Abs(ft * Ck * sum)

			if itrX == 0 && itrY == 0 {
				continue
			}

			minMaxFunc(rawValues[itrY][itrX])
		}
	}

	//histogram expansion
	if noDC {
		histExpansion(min, max, rawValues)
	}

	rawValues[0][0] = 0xff

	//set
	iterate(bounds, func(x int, y int) {
		freqIM.Image.(*im.Gray).Set(x, y, color.Gray{Y: uint8(rawValues[y][x])})
	})

	return &freqIM
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

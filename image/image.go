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

const (
	optionHistogram byte = iota
	optionCulling
	optionButterwoth
)

func (i *Image) DCT(args map[string]interface{}) *Freq.Frequencies {

	var (
		//histogram min/max
		min, max    = math.MaxFloat64, -math.MaxFloat64
		minMaxMutex sync.Mutex
		minMaxFunc  = func(val float64) {}
		option      byte
	)

	//variable changes for removal of DC signal and histogram expansion
	if _, ok := args["histogram"]; ok {
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

		option = optionHistogram
	}

	var (
		bounds = i.Image.Bounds()

		partial = Freq.NewFreq(bounds.Max.Y, bounds.Max.X)
		res     = Freq.NewFreq(bounds.Max.Y, bounds.Max.X, i.Name+"_DCT_")

		cf, order int
	)

	if v, ok := args["cutFrequency"]; ok {
		cf = v.(int)

		if v, ok = args["order"]; ok {
			order = v.(int)
			option = optionButterwoth
		} else {
			option = optionCulling
		}

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

		Freq.DCT1d(inner, bounds.Max.X, math.Sqrt(2/float64(bounds.Max.X)), Freq.Limits{Low: bounds.Min.X, High: bounds.Max.X},
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
			if option == optionHistogram {
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
			Freq.DCT1d(inner, bounds.Max.Y, math.Sqrt(2/float64(bounds.Max.Y)), Freq.Limits{Low: bounds.Min.Y, High: bounds.Max.Y},
				accessFunc, assignFunc, func(val float64) {})
			return
		}

		Freq.DCT1d(inner, bounds.Max.Y, math.Sqrt(2/float64(bounds.Max.Y)), Freq.Limits{Low: bounds.Min.Y, High: bounds.Max.Y},
			accessFunc, assignFunc, minMaxFunc)
	})

	switch option {
	case optionHistogram: //histogram expansion
		histExpansion(min, max, res.Data2D)
	case optionCulling: //culling of frequencies data
		culling.Cull(cf, res.Data2D)
		res.Filename += "CF_[" + strconv.Itoa(cf) + "]"
	case optionButterwoth:
		res.ApplyFilter(cf, order, func(x, y int) float64 { return math.Sqrt(float64(x*x + y*y)) })
		res.Filename += "BUTTERWORTH_[" + strconv.Itoa(cf) + "," + strconv.Itoa(order) + "]"
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

		Freq.IDCT1d(inner, bounds.Max.X, math.Sqrt(2/float64(bounds.Max.X)), Freq.Limits{Low: bounds.Min.X, High: bounds.Max.X},
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
			i.Image.(*im.Gray).Set(outer, inner, color.Gray{Y: uint8(math.Round(*val))})
		}

		accessFunc := func(index *int) float64 {
			//rawVal[Y<<ranged>>][X<<fixed>>]
			return partial.Data2D[*index][outer]
		}

		Freq.IDCT1d(inner, bounds.Max.Y, math.Sqrt(2/float64(bounds.Max.Y)), Freq.Limits{Low: bounds.Min.Y, High: bounds.Max.Y},
			accessFunc, assignFunc)
	})

	i.Name = freq.Filename

	return i
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

func (i *Image) Save() error {

	var (
		err error
	)

	f, err := os.Create(i.Name + "_" + strconv.Itoa(int(time.Now().Unix())) + ".png")
	if err != nil {
		return err
	}

	defer f.Close()

	err = png.Encode(f, i.Image)
	if err != nil {
		return err
	}

	return nil
}

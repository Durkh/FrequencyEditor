package audio

import (
	"github.com/Durkh/FrequencyEditor/Freq"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Audio struct {
	*audio.FloatBuffer
	format   uint16
	bitDepth int
	name     string
}

func (a *Audio) Open(path string) (err error) {

	var (
		f   *os.File
		buf *audio.IntBuffer
	)

	if f, err = os.Open(path); err != nil {
		return err
	}

	defer f.Close()

	dec := wav.NewDecoder(f)
	buf, err = dec.FullPCMBuffer()
	if err != nil {
		return err
	}

	a.FloatBuffer = buf.AsFloatBuffer()
	a.format = dec.WavAudioFormat
	a.bitDepth = int(dec.BitDepth)
	a.name = strings.Split(filepath.Base(path), ".")[0]

	return nil
}

func (a *Audio) DCT(args map[string]interface{}) *Freq.Frequencies {

	var (
		size = len(a.Data)
		res  = Freq.NewFreq(1, size, a.name+"_DCT_")

		cf, order int
	)

	if v, ok := args["cutFrequency"]; ok {
		cf = v.(int)

		if v, ok = args["order"]; ok {
			order = v.(int)
		}
	}

	iterate(size, func(i int) {

		assignFunc := func(val *float64) {
			res.Data2D[0][i] = *val
		}

		accessFunc := func(index *int) float64 {
			return a.Data[*index]
		}

		Freq.DCT1d(i, a.Format.SampleRate, math.Sqrt(2/float64(a.Format.SampleRate)), Freq.Limits{High: size},
			accessFunc, assignFunc, func(float64) {})
	})

	//var f1 = a.Format.SampleRate / (2 * len(a.Data))

	res.ApplyFilter(cf, order, func(a, b int) float64 {
		return float64(a)
	})
	res.Filename += "BUTTERWORTH_[" + strconv.Itoa(cf) + "," + strconv.Itoa(order) + "]"

	return res
}

func (a *Audio) IDCT(freq *Freq.Frequencies) Freq.Wave {

	var (
		size = len(a.Data)
	)

	a.name = freq.Filename

	iterate(size, func(i int) {

		assignFunc := func(val *float64) {
			a.Data[i] = *val
		}

		accessFunc := func(index *int) float64 {
			return freq.Data2D[0][*index]
		}

		Freq.IDCT1d(i, a.Format.SampleRate, math.Sqrt(2/float64(a.Format.SampleRate)), Freq.Limits{High: size},
			accessFunc, assignFunc)
	})

	return a
}

func iterate(len int, closure func(int)) {
	var (
		wg sync.WaitGroup

		funcChannel = make(chan int, 64)
		middleman   = func(indexes <-chan int) {
			defer wg.Done()
			for index := range indexes {
				closure(index)
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
	for itr := 0; itr < len; itr++ {
		funcChannel <- itr
	}
}

func (a *Audio) Save() (err error) {

	f, err := os.Create(a.name + "_" + strconv.Itoa(int(time.Now().Unix())) + ".wav")
	if err != nil {
		return err
	}

	enc := wav.NewEncoder(f, a.Format.SampleRate, a.bitDepth, a.Format.NumChannels, int(a.format))
	if err = enc.Write(a.FloatBuffer.AsIntBuffer()); err != nil {
		return err
	}

	if err = enc.Close(); err != nil {
		return err
	}

	defer f.Close()

	return nil
}

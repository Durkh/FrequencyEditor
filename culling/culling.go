package culling

import (
	"image"
	"runtime"
	"sync"
)

func Cull(cf int, arr [][]float64) {

	var (
		coordinates = sortFrequencies(arr, cf)

		wg sync.WaitGroup

		funcChannel = make(chan image.Point, 64)
		middleman   = func(pixels <-chan image.Point) {
			defer wg.Done()
			for pixel := range pixels {
				arr[pixel.Y][pixel.X] = 0
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

	for i := range coordinates {
		funcChannel <- coordinates[i].indexes
	}
}

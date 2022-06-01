package culling

import (
	"image"
	"math"
	"sort"
	"sync"
)

type label struct {
	val     float64
	indexes image.Point
}

func sortFrequencies(arr [][]float64, cf int) []label {

	var (
		aux = make([]label, len(arr)*len(arr[0]))
		wg  sync.WaitGroup
	)

	wg.Add(len(arr))
	for y := range arr {
		func() {
			y := y
			go func() {
				defer wg.Done()
				for x := range arr[y] {
					aux[y*len(arr[0])+x] = label{val: math.Abs(arr[y][x]), indexes: image.Point{X: x, Y: y}}
				}
			}()
		}()
	}

	wg.Wait()

	sort.Slice(aux, func(i, j int) bool {
		return aux[i].val > aux[j].val
	})

	return aux[cf+1:]
}

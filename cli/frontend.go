package cli

import (
	"fmt"
	"github.com/Durkh/FrequencyEditor/Freq"
	"github.com/Durkh/FrequencyEditor/audio"
	"github.com/Durkh/FrequencyEditor/image"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func Run() {

	if len(os.Args) < 2 {
		Exit("digite os argumentos")
	}

	var (
		args       = os.Args[1:]
		err        error
		multipart  bool
		operations []rune
		wav        Freq.Wave
		filter     = make(map[string]interface{})
	)

	for i := range args {

		if multipart {
			multipart = false
			continue
		}

		switch args[i] {

		case "-A":
			if i+1 > len(args) {
				Exit("error: digite o caminho da imagem")
			}

			a := audio.Audio{}
			if err = a.Open(args[i+1]); err != nil {
				panic(err)
			}
			wav = &a

			multipart = true
		case "-I":
			if i+1 > len(args) {
				Exit("error: digite o caminho da imagem")
			}

			im := image.Image{}
			if err = im.Open(args[i+1]); err != nil {
				panic(err)
			}
			wav = &im

			multipart = true
		case "-D": //DCT
			operations = append(operations, 'D')

			filter["histogram"] = true
		case "-F": //filter
			operations = append(operations, 'F')

			if i+1 > len(args) {
				Exit("error: digite o caminho do filtro")
			}

			multipart = true
		case "-C": //compression
			operations = append(operations, 'C')

			if i+1 > len(args) {
				Exit("error: digite a frequência de corte")
			}

			if ok, _ := regexp.MatchString(`^\[\d+]$`, args[i+1]); ok {
				cf, err := strconv.Atoi(strings.Trim(args[i+1], "[]"))
				if err != nil {
					Exit(err.Error())
				}

				filter["cutFrequency"] = &cf
			}

			delete(filter, "histogram")

			multipart = true
		}
	}

	if wav == nil {
		Exit("carregue um arquivo")
	}

	for _, v := range operations {
		switch v {
		case 'D':
			if _, ok := filter["cutFrequency"]; ok && (*filter["cutFrequency"].(*int) < 0 ||
				*filter["cutFrequency"].(*int) >
					(wav.(*image.Image).Image.Bounds().Max.X*wav.(*image.Image).Image.Bounds().Max.Y)) {

				Exit("error: frequência de corte maior que o número de amostas")
			}
			wav = wav.DCT(filter)
		case 'F':

		case 'C':

		}
	}

	if err = image.SaveImage(*wav.(*image.Image)); err != nil {
		Exit(err.Error())
	}

}

func Exit(err string) {
	fmt.Println(err)
	os.Exit(1)
}

package cli

import (
	"fmt"
	"github.com/Durkh/FrequencyEditor/Freq"
	"github.com/Durkh/FrequencyEditor/audio"
	"github.com/Durkh/FrequencyEditor/image"
	"math"
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
		options    = make(map[string]interface{})
	)

	for i := range args {

		if multipart {
			multipart = false
			continue
		}

		switch args[i] {

		case "-A":
			if i+1 > len(args) {
				Exit("error: digite o caminho do audio")
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

			options["histogram"] = true
		case "-F": //filter
			operations = append(operations, 'F')

			if i+1 > len(args)-1 {
				Exit("error: digite a frequência de corte")
			}

			if ok, _ := regexp.MatchString(`^\[\d+;\d+]$`, args[i+1]); ok {
				aux := strings.FieldsFunc(strings.Trim(args[i+1], "[]"), func(r rune) bool {
					return r == ';'
				})

				var cf, order int

				cf, err = strconv.Atoi(aux[0])
				if err != nil {
					Exit(err.Error())
				}
				order, err = strconv.Atoi(aux[1])
				if err != nil {
					Exit(err.Error())
				}

				options["cutFrequency"] = cf
				options["order"] = order

				delete(options, "histogram")
			} else {
				Exit("error: frequencia inválida")
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

				options["cutFrequency"] = cf

				delete(options, "histogram")
			} else {
				Exit("error: frequencia inválida")
			}

			multipart = true

		case "DEBUG":
			operations = append(operations, 'Z')
		}

	}

	if wav == nil {
		Exit("carregue um arquivo")
	}

	for _, v := range operations {
		switch v {
		case 'D':
			freq := wav.DCT(options)
			wav.(*image.Image).Image = freq.ToGray()
			wav.(*image.Image).Name = freq.Filename

		case 'F':

			switch wav.(type) {
			case *audio.Audio:
				if cf, ok := options["cutFrequency"]; ok && (cf.(int) < 0 || cf.(int) > wav.(*audio.Audio).Format.SampleRate) {
					Exit("error: frequência de corte maior que a frequência máxima")
				}
			case *image.Image:
				if cf, ok := options["cutFrequency"]; ok && (cf.(int) < 0 || float64(cf.(int)) >
					math.Sqrt(math.Pow(float64(wav.(*image.Image).Image.Bounds().Max.X), 2)+
						math.Pow(float64(wav.(*image.Image).Image.Bounds().Max.Y), 2))) {

					Exit("error: frequência de corte maior que o número de amostras")
				}
			}

			wav.IDCT(wav.DCT(options))
		case 'C':

			if cf, ok := options["cutFrequency"]; ok && (cf.(int) < 0 || cf.(int) >
				(wav.(*image.Image).Image.Bounds().Max.X*wav.(*image.Image).Image.Bounds().Max.Y)-1) {

				Exit("error: frequência de corte maior que o número de amostras")
			}

			wav.IDCT(wav.DCT(options))

		case 'Z':
			wav.IDCT(wav.DCT(map[string]interface{}{}))
		}

	}

	if err = wav.Save(); err != nil {
		Exit(err.Error())
	}

}

func Exit(err string) {
	fmt.Println(err)
	os.Exit(1)
}

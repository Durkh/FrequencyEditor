package audio

import "github.com/Durkh/FrequencyEditor/Freq"

type Audio struct {
	Data []byte
}

func (a *Audio) Open(path string) (err error) {

	return nil
}

func (a *Audio) DCT(args map[string]interface{}) Freq.Wave {

	return a
}

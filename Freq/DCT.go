package Freq

type Wave interface {
	DCT(map[string]interface{}) Wave
	Open(string) error
}

package qc_test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/alx696/go-kc/qc"
)

func TestEncode(t *testing.T) {
	e := qc.Encode("/home/m/图片/test.jpg", "https://lilu.red/app/pl/?id=12D3KooWPrqbPwKZ6Gyf7SyUMBAfEDQaNecKgJvBhkc1mha11bUN")
	if e != nil {
		log.Fatalln(e)
	}
}

func TestDecode(t *testing.T) {
	data, e := ioutil.ReadFile("/home/m/图片/test.jpg")
	if e != nil {
		log.Fatalln(e)
	}

	txt, e := qc.DecodeBytes(data)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(txt)
}

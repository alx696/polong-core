package qc

import (
	"bytes"
	"image"
	"image/jpeg"
	"log"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// Encode 二维码编码
func Encode(path, text string) error {
	log.Println("二维码编码", path, text)

	hints := make(map[gozxing.EncodeHintType]interface{})
	hints[gozxing.EncodeHintType_MARGIN] = 0

	qrWriter := qrcode.NewQRCodeWriter()

	bm, e := qrWriter.Encode(text, gozxing.BarcodeFormat_QR_CODE, 128, 128, hints)
	if e != nil {
		return e
	}

	file, e := os.Create(path)
	if e != nil {
		return e
	}
	defer file.Close()
	e = jpeg.Encode(file, bm, nil)
	if e != nil {
		return e
	}

	return nil
}

// DecodeBytes 二维码解码图片字节玛
func DecodeBytes(data []byte) (string, error) {
	img, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return "", e
	}

	// prepare BinaryBitmap
	bmp, e := gozxing.NewBinaryBitmapFromImage(img)
	if e != nil {
		return "", e
	}

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, e := qrReader.Decode(bmp, nil)
	if e != nil {
		return "", e
	}

	return result.GetText(), nil
}

// DecodeYUV 二维码解码YUV
func DecodeYUV(yuvData []byte, dataWidth int, dataHeight int) (string, error) {
	luminanceSource, e := gozxing.NewPlanarYUVLuminanceSource(yuvData, dataWidth, dataHeight, 0, 0, dataWidth, dataHeight, false)
	if e != nil {
		return "", e
	}

	bmp, e := gozxing.NewBinaryBitmap(gozxing.NewHybridBinarizer(luminanceSource))
	if e != nil {
		return "", e
	}

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, e := qrReader.Decode(bmp, nil)
	if e != nil {
		return "", e
	}

	return result.GetText(), nil
}

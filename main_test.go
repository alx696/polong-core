package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/alx696/go-tool"
)

var serviceURL string

func TestMain(m *testing.M) {
	serviceURL = "http://127.0.0.1:10001"

	os.Exit(m.Run())
}

func TestInfoPost(t *testing.T) {
	fileFiled := map[string]tool.FileFiled{}
	textField := map[string]string{"cid": "12D3KooWBZzTWvHJYAzmtgmY2BXwX8puz6wP3WW7Q2XAerGD3vfa"}
	contentType, formDataBuffer, _ := tool.FormData(fileFiled, textField)
	statusCode, bodyBytes, e := tool.RequestFormData(fmt.Sprint(serviceURL, "/api1/info"), "POST", contentType, formDataBuffer)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), statusCode)
}

func TestInfoPut(t *testing.T) {
	fileFiled := map[string]tool.FileFiled{}
	textField := map[string]string{"cid": "12D3KooWBZzTWvHJYAzmtgmY2BXwX8puz6wP3WW7Q2XAerGD3vfa", "name": "调试1", "photo_base64": ""}
	contentType, formDataBuffer, _ := tool.FormData(fileFiled, textField)
	statusCode, bodyBytes, e := tool.RequestFormData(fmt.Sprint(serviceURL, "/api1/info"), "PUT", contentType, formDataBuffer)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), statusCode)
}

func TestInfoGet(t *testing.T) {
	resp, e := http.Get(fmt.Sprint(serviceURL, "/api1/info"))
	if e != nil {
		log.Fatalln(e)
	}
	defer resp.Body.Close()

	bodyBytes, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), resp.StatusCode)
}

func TestFilePost(t *testing.T) {
	fileBytes, _ := ioutil.ReadFile("/home/m/图片/test.png")
	fileFiled := map[string]tool.FileFiled{"file": {FileName: "测试.png", Data: fileBytes}}
	textField := map[string]string{}
	contentType, formDataBuffer, _ := tool.FormData(fileFiled, textField)
	statusCode, bodyBytes, e := tool.RequestFormData(fmt.Sprint(serviceURL, "/api1/file"), "POST", contentType, formDataBuffer)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), statusCode)
}

func TestFileGet(t *testing.T) {
	resp, e := http.Get(fmt.Sprint(serviceURL, "/api1/file?id=", "f36fe7b8-3030-478c-ac8c-0f4dfc1fa67c"))
	if e != nil {
		log.Fatalln(e)
	}
	defer resp.Body.Close()

	bodyBytes, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(len(bodyBytes), resp.StatusCode)
}

func TestMessagePost(t *testing.T) {
	fileFiled := map[string]tool.FileFiled{}
	textField := map[string]string{"cid": "12D3KooWNFDpNsYZjAGk3WKpyLCU8ANGAEfo7hNMtHTQ8yZrbcrp", "text": "你好", "file_id": "", "file_name": "测试.png"}
	// textField := map[string]string{"cid": "12D3KooWNFDpNsYZjAGk3WKpyLCU8ANGAEfo7hNMtHTQ8yZrbcrp", "text": "", "file_id": "f36fe7b8-3030-478c-ac8c-0f4dfc1fa67c", "file_name": "测试.png"}
	contentType, formDataBuffer, _ := tool.FormData(fileFiled, textField)
	statusCode, bodyBytes, e := tool.RequestFormData(fmt.Sprint(serviceURL, "/api1/message"), "POST", contentType, formDataBuffer)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), statusCode)
}

func TestMessageGet(t *testing.T) {
	resp, e := http.Get(fmt.Sprint(serviceURL, "/api1/message?cid=", "12D3KooWNFDpNsYZjAGk3WKpyLCU8ANGAEfo7hNMtHTQ8yZrbcrp"))
	if e != nil {
		log.Fatalln(e)
	}
	defer resp.Body.Close()

	bodyBytes, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), resp.StatusCode)
}

func TestMessageDelete(t *testing.T) {
	req, e := http.NewRequest(http.MethodDelete, fmt.Sprint(serviceURL, "/api1/message?cid=", "12D3KooWNFDpNsYZjAGk3WKpyLCU8ANGAEfo7hNMtHTQ8yZrbcrp"), nil)
	if e != nil {
		log.Fatalln(e)
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		log.Fatalln(e)
	}
	defer resp.Body.Close()

	bodyBytes, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), resp.StatusCode)
}

func TestDNSGet(t *testing.T) {
	resp, e := http.Get(fmt.Sprint(serviceURL, "/api1/dns?name=pl.app.lilu.red&type=16"))
	if e != nil {
		log.Fatalln(e)
	}
	defer resp.Body.Close()

	bodyBytes, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println(string(bodyBytes), resp.StatusCode)
}

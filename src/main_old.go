package reptile

import (
	"encoding/json"
	"fmt"
	"github.com/axgle/mahonia"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
)

func main2() {
	ch := make(chan string)
	codes := [...]string {"300014"}
	for _, code := range codes {
		//go fetch(code, ch)
		fetch2(code, ch)
	}
}

func fetch2(code string, ch chan<- string) {
	//根据url获取内容
	url := fmt.Sprintf("http://data.eastmoney.com/DataCenter_V3/jgdy/gsjsdy.ashx?pagesize=50&page=1&param=&sortRule=-1&sortType=0&code=%s", code)
	resp, err := http.Get(url)
	if err != nil {
		ch <- fmt.Sprintf("HTTP Error: %v", err)
		resp.Body.Close()
		return
	}
	//读取响应体内容
	var surveyResult surveyResult
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ch <- fmt.Sprintf("Read Response Error: %v", err)
		resp.Body.Close()
		return
	}
	//解码为utf-8
	body = []byte(mahonia.NewDecoder("gbk").ConvertString(string(body)))
	err = json.Unmarshal(body, &surveyResult)
	if err != nil {
		ch <- fmt.Sprintf("Decode Error: %v", err)
		resp.Body.Close()
		return
	}
	resp.Body.Close()
	for _, sr := range surveyResult.Data {
		if sr.Description == "特定对象调研" {
			resp, err = http.Get(fmt.Sprintf("http://data.eastmoney.com/jgdy/dyxx/%s,%s.html", code, sr.StartDate))
			if err != nil {
				ch <- fmt.Sprintf("读取调研详情出错: %v\ncode:%s\tdate:%s\n", err, code, sr.StartDate)
				resp.Body.Close()
				return
			}
			_, err := html.Parse(resp.Body)
			if err != nil {
				ch <- fmt.Sprintf("parse html error: %v\ncode:%s\tdate:%s\n", err, code, sr.StartDate)
				resp.Body.Close()
				return
			}
			resp.Body.Close()
		}
	}

	//ch <- fmt.Sprint()
}
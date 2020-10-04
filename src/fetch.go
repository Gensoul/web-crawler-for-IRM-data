package reptile

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/axgle/mahonia"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	attr        map[string]int32
	dateAttr    map[string]attr
	companyAttr map[string]dateAttr
)

const layout = "2006-01-02"

var (
	wg			sync.WaitGroup
	ch			chan companyAttr
	startDate	time.Time
	endDate		time.Time
	result		companyAttr
	reg			*regexp.Regexp
	companySet	map[string]interface{}
	years		[]string
	fields		[]string
)

func init() {
	ch = make(chan companyAttr, 10)
	startDate, _ = time.Parse(layout, "2010-01-01")
	endDate, _ = time.Parse(layout, "2019-12-31")
	result = make(companyAttr)
	reg, _ = regexp.Compile("[,，、]")
	companySet = make(map[string]interface{})
	years = []string {"2010", "2011", "2012", "2013", "2014", "2015", "2016", "2017", "2018", "2019"}
	fields = []string {"股票代码", "会计年度", "接待次数", "接待机构数", "接待机构人员数", "基金管理公司", "证券公司", "保险资产管理公司", "寿险公司", "投资公司", "资产管理公司", "期货经纪公司", "其它"}
}

//获取上市公司列表
//根据巨潮信息网提供的API获取
func getCompanyInfo() ([]string, error){
	//先获取token
	token, err := getToken()
	if err != nil {
		return nil, err
	}
	//获取公司列表
	const plateType = "137001"//分类代码
	plateCodes := []string{"012002", "012003", "012015"}//板块代码列表，分别表示深市中小板、深市主板、深市创
	result := make([]string, 0, 100000)
	for _, plateCode := range plateCodes {
		url := fmt.Sprintf("http://webapi.cninfo.com.cn/api/stock/p_public0004?access_token=%s&platetype=%s&platecode=%s", token, plateType, plateCode)
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		var infoResult companyInfoResult
		if err = json.NewDecoder(resp.Body).Decode(&infoResult); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		if infoResult.ResultCode != 200 {
			return nil, fmt.Errorf("Get Company Error: %s\n", infoResult.ResultMsg)
		}

		for _, info := range infoResult.Records {
			result = append(result, info.SECCODE)
		}
	}
	return result, nil
}

//获取巨潮网的token
func getToken() (string, error) {
	const (
		accessKey = "a726daee6e7d4a89b8e5aa5fc4cdc5c7"
		accessSecret = "5473f912ec784f758a17bb078243ebde"
	)
	url := fmt.Sprintf("http://webapi.cninfo.com.cn/api-cloud-platform/oauth2/token?grant_type=client_credentials&client_id=%s&client_secret=%s", accessKey, accessSecret)
	resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var t token
	if err = json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

//爬取公司调研信息并统计
func Fetch() {
	//先获取深市公司列表，存到map中
	companyList, err := getCompanyInfo()
	if err != nil {
		fmt.Printf("Get Company Info Error: %v\n", err)
		return
	}
	for _, code := range companyList {
		companySet[code] = nil
	}
	//爬取公司信息
	wg.Add(1)
	go fetchSurveyRecords(startDate, endDate, 500, 1)
	go func() {
		wg.Wait()
		close(ch)
		fmt.Println("close")
	}()
	var cnt int
	for tmp := range ch {
		for code, dateMap := range tmp {
			for date, attrMap := range dateMap {
				if _, ok := result[code]; !ok {
					result[code] = make(dateAttr)
				}
				if _, ok := result[code][date]; !ok {
					result[code][date] = make(attr)
				}
				for a, v := range attrMap {
					result[code][date][a] = result[code][date][a] + v
				}
			}
		}

		cnt++
		if cnt % 10 == 0 {
			export("/Users/bytedance/Downloads/reptile/result.csv")
		}
	}
	export("/Users/bytedance/Downloads/reptile/result.csv")
}

//爬取网页上的所有调研记录
//@param startDate 大于等于开始日期
//@param endDate 小于等于结束日期
//@param pageSize 单页数目
//@param page 页码
func fetchSurveyRecords(startDate time.Time, endDate time.Time, pageSize int, page int) {
	defer wg.Done()
	url := fmt.Sprintf("http://data.eastmoney.com/DataCenter_V3/jgdy/gsjsdy.ashx?pagesize=%d&page=%d&param=&sortRule=-1&sortType=0", pageSize, page)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("HTTP Get Error = %v, url = %s\n", err, url)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Read Response Error: %v, page = %d", err, page)
	}
	//解码为utf-8
	body = []byte(mahonia.NewDecoder("gbk").ConvertString(string(body)))
	var surveyResult surveyResult
	if err = json.Unmarshal(body, &surveyResult); err != nil {
		fmt.Printf("Json Decode Error = %v, url = %s, json = %s\n", err, url, resp.Body)
		return
	}
	if len(surveyResult.Data) > 0 {
		wg.Add(1)
		go fetchSurveyRecords(startDate, endDate, pageSize, page + 1)
	}
	m := make(companyAttr)
	for _, survey := range surveyResult.Data {
		date, _ := time.Parse(layout, survey.StartDate)
		if _, ok := companySet[survey.SCode]; !ok {
			continue
		}
		if date.Before(startDate) || date.After(endDate) {
			continue
		}
		if strings.Contains(survey.Description, "特定对象调研") || strings.Contains(survey.Description, "实地调研") {
			var cnt int
			for {
				if err := FetchSurveyDetails(survey.SCode, survey.StartDate, strconv.FormatInt(int64(date.Year()), 10), m); err == nil {
					break
				}
				cnt++
				fmt.Printf("retry %d, code = %s, date = %s", cnt, survey.SCode, survey.StartDate)
			}
		}
	}
	ch <- m
	fmt.Printf("fetch success! page = %d\n", page)
}
//获取调研详情
//@param code 股票代码
//@param date 日期
func FetchSurveyDetails(code string, date string, year string, m companyAttr) error {
	resp, err := http.Get(fmt.Sprintf("http://data.eastmoney.com/jgdy/dyxx/%s,%s.html", code, date))
	if err != nil {
		fmt.Printf("Get Detail Error: %v\tcode: %s\tdate: %s\n", err, code, date)
		return err
	}
	defer resp.Body.Close()
	//读取响应并转码
	byteBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Read Response Error: %v\n, code = %s, date = %s", err, code, date)
		return err
	}
	body := mahonia.NewDecoder("gbk").ConvertString(string(byteBody))
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		fmt.Printf("Read Doc From String Error: %v\n", err)
		return err
	}
	//提取详细信息
	if _, ok := m[code]; !ok {
		m[code] = make(dateAttr)
	}
	if _, ok := m[code][year]; !ok {
		m[code][year] = make(attr)
	}
	m[code][year]["接待次数"]++
	doc.Find("div.Content").Find("table").Find("tbody").Find("tr").Each(func(i int, s *goquery.Selection) {
		tds := s.Children()
		m[code][year]["接待机构数"]++
		m[code][year][tds.Eq(2).Text()]++
		people := tds.Eq(3).Text()
		if l := len(reg.Split(people, -1)); l > 0 {
			m[code][year]["接待机构人员数"] += int32(l)
		} else {
			m[code][year]["接待机构人员数"]++
		}
	})
	return nil
}

//导出记录
func export(filename string) {
	// 创建文件
	newFile, err := os.Create(filename)
	if err != nil {
		fmt.Println("创建文件失败")
		return
	}
	defer newFile.Close()
	// 写入UTF-8
	newFile.WriteString("\xEF\xBB\xBF") // 写入UTF-8 BOM，防止中文乱码
	w := csv.NewWriter(newFile)
	w.Write(fields)
	tmpResult := [][]string{}
	for code, _ := range companySet {
		for _, year := range years {
			tmp := make([]string, 2)
			tmp[0] = code
			tmp[1] = year
			for _, field := range fields[2:] {
				tmp = append(tmp, strconv.FormatInt(int64(result[code][year][field]), 10))
			}
			tmpResult = append(tmpResult, tmp)
		}
	}
	w.WriteAll(tmpResult)
	w.Flush()
	fmt.Println("导出成功")
}
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type BookBuilder struct {
	bookISBN       string
	bookName       string
	bookImgUrl     string
	bookStar       string
	bookStarPeople string
	bookComment    string
	bookAuthor     string
	bookTranslator string
	bookPublisher  string
	bookYear       string
	bookPrice      float64
	described      string
}

type Book struct {
	BookISBN         string  `json:"ISBN"`
	BookName         string  `json:"name"`
	BookAuthor       string  `json:"author"`
	BookPublishHouse string  `json:"publishHouse"`
	BookPrice        float64 `json:"price"`
	BookDescribed    string  `json:"described"`
	BookImgUrl       string  `json:"imgUrl"`
}

func (my BookBuilder) Build() *Book {
	return &Book{
		BookISBN:         trimAll(my.bookISBN),
		BookName:         trimAll(my.bookName),
		BookAuthor:       trimAll(my.bookAuthor),
		BookPublishHouse: trimAll(my.bookPublisher),
		BookPrice:        my.bookPrice,
		BookDescribed:    trimAll(my.described),
		BookImgUrl:       trimAll(my.bookImgUrl),
	}
}

func main() {
	ok := make(chan int, 12)
	pageSize := 25
	basicHeaders := map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36",
	}
	for index := range 10 {
		requestUrl := fmt.Sprintf("https://book.douban.com/top250?start=%d", index*pageSize)
		go asyncGetBookAndToService(ok, &basicHeaders, requestUrl, index)
		time.Sleep(time.Second * 2)
	}
	for index := range 10 {
		fmt.Printf("Line: %d 第%d页任务已完成\n", index, <-ok)
	}
	close(ok)
}
func asyncGetBookAndToService(ok chan int, basicHeader *map[string]string, url string, index int) {
	fmt.Printf("开始爬取第%d页,URL为 %s\n", index, url)
	bookBuilders := robotAndGetBookBuilders(basicHeader, url)
	sendBookToDB(bookBuilders)
	ok <- index
}
func robotAndGetBookBuilders(basicHeader *map[string]string, url string) (bookBuilders []BookBuilder) {
	client := http.Client{}
	mainDomReq, err := http.NewRequest("GET", url, nil)
	panicIfErr(err)
	for k, v := range *basicHeader {
		mainDomReq.Header.Add(k, v)
	}
	mainDomRes, err := client.Do(mainDomReq)
	panicIfErr(err)
	defer mainDomRes.Body.Close()
	mainDom, err := goquery.NewDocumentFromReader(mainDomRes.Body)
	panicIfErr(err)
	mainDom.Find("table").Each(func(i int, s *goquery.Selection) {

		bookBuilder := BookBuilder{}

		bookBuilder.bookName = trimAll(strings.Replace(s.Find("div.pl2 a").Text(), "\"", "", -1))
		fmt.Println("正在爬取", bookBuilder.bookName)

		bookBuilder.bookImgUrl, _ = s.Find("img").Attr("src")

		bookBuilder.bookStar = s.Find("div.star span.rating_nums").Text()

		starPeopleElement := s.Find("div.star span.pl")
		starPeopleText := strings.TrimSpace(starPeopleElement.Text())
		re := regexp.MustCompile(`\d+`)
		matches := re.FindStringSubmatch(starPeopleText)
		if len(matches) > 0 {
			bookBuilder.bookStarPeople = matches[0]
		} else {
			bookBuilder.bookStarPeople = ""
		}

		bookBuilder.bookComment = ""
		if quote := s.Find("p.quote span"); quote != nil {
			bookBuilder.bookComment = strings.TrimSpace(quote.Text())
		}

		bookInfoText := strings.TrimSpace(s.Find("p.pl").Text())
		infoUnits := strings.Split(bookInfoText, "/")

		rawAuthor := infoUnits[0]
		bookBuilder.bookAuthor = strings.Replace(rawAuthor, " 著", "", -1)

		if len(infoUnits) == 5 {
			bookBuilder.bookTranslator = infoUnits[1]
		}

		bookBuilder.bookPublisher = infoUnits[len(infoUnits)-3]

		bookBuilder.bookYear = infoUnits[len(infoUnits)-2]
		rawBookPrice := infoUnits[len(infoUnits)-1]
		numberReg := regexp.MustCompile(`[+\-]?(?:(?:0|[1-9]\d*)(?:\.\d*)?|\.\d+)(?:\d[eE][+\-]?\d+)?`)
		bookPrice, _ := strconv.ParseFloat(numberReg.FindString(rawBookPrice), 64)
		bookBuilder.bookPrice = bookPrice

		moreInfoUrl, _ := s.Find("div.pl2 a").Attr("href")
		moreInfoReq, _ := http.NewRequest("GET", moreInfoUrl, nil)
		for k, v := range *basicHeader {
			moreInfoReq.Header.Add(k, v)
		}
		moreInfoRes, err := client.Do(moreInfoReq)
		panicIfErr(err)
		defer moreInfoRes.Body.Close()

		moreInfoDoc, err := goquery.NewDocumentFromReader(moreInfoRes.Body)
		panicIfErr(err)

		infoElement := moreInfoDoc.Find("div#info")
		infoText := infoElement.Text()
		if strings.Contains(infoText, "ISBN") {
			bookBuilder.bookISBN = strings.TrimSpace(strings.Split(infoText, "ISBN:")[1])
		} else if strings.Contains(infoText, "统一书号") {
			bookBuilder.bookISBN = strings.TrimSpace(strings.Split(infoText, "统一书号:")[1])
		}

		bookBuilder.described = strings.TrimSpace(moreInfoDoc.Find("div.related_info p").First().Text())

		bookBuilders = append(bookBuilders, bookBuilder)

	})
	return
}
func sendBookToDB(bookBuilders []BookBuilder) {
	chans := make(chan int, len(bookBuilders))
	for _, bookBuilder := range bookBuilders {
		go asyncSendBookRequest(chans, bookBuilder.Build())
	}
	for i := 0; i < len(bookBuilders); i++ {
		<-chans
	}
	close(chans)
}
func asyncSendBookRequest(chans chan int, book *Book) {
	baseUrl := "http://localhost:3000/stock/addbook"
	fmt.Printf(`
	存入的书本信息如下：
	ISBN：%s
	书籍名称：%s
	作者：%s
	出版社名称：%s
	价格：%f
	描述：%s
	图片地址：%s`, book.BookISBN, book.BookName, book.BookAuthor, book.BookPublishHouse, book.BookPrice, book.BookDescribed, book.BookImgUrl)
	fmt.Println("")
	reqBodys, _ := json.Marshal(*book)
	res, err := http.Post(baseUrl, "application/json; charset=utf-8", bytes.NewReader(reqBodys))
	panicIfErr(err)
	resBodys, err := io.ReadAll(res.Body)
	panicIfErr(err)
	fmt.Println(string(resBodys))
	chans <- 0
}
func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
func trimAll(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

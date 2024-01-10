package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"time"

	"golang.org/x/net/html"
)

func getHtmlPage(url, userAgent string) ([]byte, error) {
	// функция получения ресурсов по указанному адресу url с использованием User-Agent
	// возвращает загруженный HTML контент
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Cannot create new request  %s, error: %v\n", url, err)
		return nil, err
	}

	// с User-agent по умолчанию контент не отдается, используем свой
	req.Header.Set("User-Agent", userAgent)

	// Отправляем запрос
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error with GET request: %v\n", err)
		return nil, err
	}

	defer resp.Body.Close()

	// Получаем контент и возвращаем его, как результат работы функции
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error ReadAll")
		return nil, err
	}
	return body, nil
}

func getArticle(body []byte, tag, keyattr, value string) string {
	// Функция получения текста статьи из html контента
	// Текст достается из тега tag с атрибутом keyattr, значение атрибута value
	// Возвращает содержание статьи
	tkn := html.NewTokenizer(bytes.NewReader(body))
	depth := 0
	var article string
	block := ""
	errorCode := false

	// Проходим по всему дереву тегов (пока не встретится html.ErrorToken)
	for !errorCode {
		tt := tkn.Next()
		switch tt {
		case html.ErrorToken:
			errorCode = true
		case html.TextToken:
			if depth > 0 {
				block += string(tkn.Text()) // Если внутри нужного тега, забираем текст из блока
			}
		case html.StartTagToken, html.EndTagToken:
			tn, tAttr := tkn.TagName()
			if string(tn) == tag { // выбираем нужный tag
				if tAttr {
					key, attr, _ := tkn.TagAttr()
					if tt == html.StartTagToken && string(key) == keyattr && string(attr) == value {
						depth++ // нужный тег открывается
					}
				} else if tt == html.EndTagToken && depth >= 1 {
					depth--
					article += block // Когда блок закрывается, добавляем текст из блока в основной текст
					block = ""
				}
			}
		}
	}
	return article
}

func getHash(article string) uint32 {
	// Получение хеша из содержания статьи

	hArticle := crc32.NewIEEE()
	hArticle.Write([]byte(article))
	return hArticle.Sum32()
}

func main() {
	var urlList string
	var userAgent string
	var newsage float64

	// XML structure of RSS
	type RiaRss struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Title     string `xml:"title"`
			Link      string `xml:"link"`
			Language  string `xml:"language"`
			Copyright string `xml:"copyright"`
			Item      []struct {
				Title     string `xml:"title"`
				Link      string `xml:"link"`
				Guid      string `xml:"guid"`
				Priority  string `xml:"rian:priority"`
				Pubdate   string `xml:"pubDate"`
				Type      string `xml:"rian:type"`
				Category  string `xml:"category"`
				Enclosure string `xml:"enclosure"`
			} `xml:"item"`
		} `xml:"channel"`
	}

	//Структуры для файла с хешами статей
	type ArticleH struct {
		URL     string
		Hash    uint32
		Created time.Time
	}
	// Ключи для командной строки
	flag.StringVar(&urlList, "xml", "0", "XML with list of the articles")
	flag.Float64Var(&newsage, "dt", 259200, "Time in seconds to verify changing in news (3 days by default)")
	flag.StringVar(&userAgent, "uagent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit", "User Agent")

	flag.Parse()

	// Если не указан xml выходим: проверять нечего
	if urlList == "0" {
		fmt.Println(("XML must be specified"))
		return
	}

	// Проверка ссылок на статьи из xml файла
	rss := new(RiaRss)
	// Получаем текст RSS
	body, err := getHtmlPage(urlList, userAgent)
	if err != nil {
		fmt.Printf("Error getHtmlPage - %v\n", err)
	}
	// Разбираем полученный RSS
	err1 := xml.Unmarshal([]byte(body), rss)
	if err != nil {
		fmt.Printf("error: %v", err1)
		return
	}

	// Пребираем все ссылки в RSS
	for _, value := range rss.Channel.Item {
		fmt.Println("========>", value.Link)
		body, err := getHtmlPage(value.Link, userAgent)
		if err != nil {
			fmt.Printf("Error getHtmlPage - %v\n", err)
		}
		// Получаем заголовок и текст статьи
		article := getArticle(body, "div", "class", "article__title") + "\n"
		article += getArticle(body, "div", "class", "article__text")
		articleHash := getHash(article)
	}
	// err = os.WriteFile("error.html", []byte(html_head+htmlerr+"</body>"), 0644)
	// if err != nil {
	// fmt.Printf("Error write HTML file - %v\n", err)
	// }
}

package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"strconv"

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
	var url string
	var urlList string
	var userAgent string

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

	// Ключи для командной строки
	flag.StringVar(&url, "url", "0", "URL of the article")
	flag.StringVar(&urlList, "xml", "0", "XML with list of the articles")
	flag.StringVar(&userAgent, "uagent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit", "User Agent")

	flag.Parse()

	// Если не указан url и xml выходим: проверять нечего
	if url == "0" && urlList == "0" {
		fmt.Println(("URL or XML must be specified"))
		return
	}

	// Проверка для единичного адреса
	if url != "0" {
		// Получаем html контент
		body, err := getHtmlPage(url, userAgent)
		if err != nil {
			fmt.Printf("Error getHtmlPage - %v\n", err)
		}
		// Получаем заголовок и текст статьи
		article := getArticle(body, "div", "class", "article__title") + "\n"
		article += getArticle(body, "div", "class", "article__text")
		articleHash := getHash(article)

	} else if urlList != "0" {
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

		var article_err string
		totalLng := 0
		// Пребираем все ссылки в RSS
		for _, value := range rss.Channel.Item {
			fmt.Println("========>", value.Link)
			htmlerr += "<p>Link to the article: <a href='" + value.Link + "'>" + value.Link + "</a></p>\n"
			// Получаем HTML контент
			body, err := getHtmlPage(value.Link, userAgent)
			if err != nil {
				fmt.Printf("Error getHtmlPage - %v\n", err)
			}
			// Получаем текст статьи
			article := getArticle(body, "div", "class", "article__text")
			articleLen := len(article)
			fmt.Println("Total length: ", articleLen)
			htmlerr += "<p>Article length: " + strconv.Itoa(articleLen) + "</p>\n"
			totalLng += articleLen
			opt.Article = article
			sperror, err_sp := speller(opt)

			// Если есть ошибки в тексте, готовим вывод результата
			if len(sperror) > 0 {
				article_err = addtags(article, subs_cl, sperror)
				for _, v := range sperror {
					fmt.Printf("Incorrect world: %v, pos: %v, len: %v, error: %v\n", v.Word, v.Pos, v.Len, errorCode[v.Code])
					htmlerr += fmt.Sprintf("<p>Incorrect world: %v, pos: %v, len: %v, error: %v</p>\n", v.Word, v.Pos, v.Len, errorCode[v.Code])
				}
				fmt.Println("Article with errors:", article_err)
				htmlerr += "<p>" + article_err + "</p>\n"
			}
			if err_sp != nil {
				fmt.Printf("Error speller - %v\n", err_sp)
			}
			htmlerr += "<br><br>\n"
		}
		htmlerr += "<p>Total article length: " + strconv.Itoa(totalLng) + "</p>\n"
		err = os.WriteFile("error.html", []byte(html_head+htmlerr+"</body>"), 0644)
		if err != nil {
			fmt.Printf("Error write HTML file - %v\n", err)
		}
	}
}

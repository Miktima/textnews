package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

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

//Структура для файла с хешами статей
type ArticleH struct {
	URL     string
	Hash    uint32
	Created time.Time
}

//Структура для данных статей
type NewsData struct {
	URL, Article string
}

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

func inSlice(tSlice []ArticleH, url string) bool {
	for _, v := range tSlice {
		if v.URL == url {
			return true
		}
	}
	return false
}

func main() {
	var urlList string
	var userAgent string
	var newsage float64

	// Ключи для командной строки
	flag.StringVar(&urlList, "xml", "0", "XML with list of the articles")
	flag.Float64Var(&newsage, "dt", 259200, "Time in seconds to verify changing in news (3 days by default)")
	flag.StringVar(&userAgent, "uagent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit", "User Agent")

	flag.Parse()

	var listHash []ArticleH
	var listData []NewsData

	path, _ := os.Executable()
	path = path[:strings.LastIndex(path, "/")+1]
	fmt.Println("Path: ", path)

	// Читаем файл с хешами статей и удаляем файл
	if _, err := os.Stat(path + "/hashes.json"); err == nil {
		// Open our jsonFile
		byteValue, err := os.ReadFile(path + "/hashes.json")
		// if we os.ReadFile returns an error then handle it
		if err != nil {
			fmt.Println(err)
		}
		// defer the closing of our jsonFile so that we can parse it later on
		// var listHash []ArticleH
		err = json.Unmarshal(byteValue, &listHash)
		if err != nil {
			fmt.Println(err)
		}
		// Удаляем файл
		err = os.Remove(path + "/hashes.json")
		if err != nil {
			fmt.Println(err)
		}
	}

	// Если статья достаточно старая (возраст больше newsage) проверяем текущий hash
	// И, если текущий хеш отличается от первоначального, то записываем эту статью, как правильную

	var d time.Duration
	var article string
	var data NewsData

	for _, value := range listHash {
		d = time.Since(value.Created)
		if d.Seconds() > newsage {
			body, err := getHtmlPage(value.URL, userAgent)
			if err != nil {
				fmt.Printf("Error getHtmlPage - %v\n", err)
			}
			// Получаем заголовок и текст статьи
			article = getArticle(body, "div", "class", "article__title") + "\n"
			article += getArticle(body, "div", "class", "article__text")
			// Сравниваем hash статьи
			fmt.Println("DATA Checked========>", value.URL)
			if value.Hash != getHash(article) {
				data.URL = value.URL
				data.Article = article
				listData = append(listData, data)
				fmt.Println("DATA Stored========>", value.URL)
			}
		}
	}

	//записываем данные в файл, если они есть
	if len(listData) > 0 {
		f, err := os.OpenFile(path+"/datanews.json", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			fmt.Printf("Error opening to datanews.json file - %v\n", err)
		}
		defer f.Close()

		arData, _ := json.MarshalIndent(listData, "", " ")
		_, err = f.Write(arData)
		if err != nil {
			fmt.Printf("Error write Article data - %v\n", err)
		}
	}

	// Если не указан xml выходим
	if urlList == "0" {
		fmt.Println(("XML не указан!!!"))
	} else {
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

		var articleHash ArticleH
		// Перебираем все ссылки в RSS
		for _, value := range rss.Channel.Item {
			if !inSlice(listHash, value.Link) {
				fmt.Println("HASH========>", value.Link)
				body, err := getHtmlPage(value.Link, userAgent)
				if err != nil {
					fmt.Printf("Error getHtmlPage - %v\n", err)
				}
				// Получаем заголовок и текст статьи
				article = getArticle(body, "div", "class", "article__title") + "\n"
				article += getArticle(body, "div", "class", "article__text")
				// Получаем hash статьи
				articleHash.URL = value.Link
				articleHash.Hash = getHash(article)
				articleHash.Created, _ = time.Parse(time.RFC1123Z, value.Pubdate)
				listHash = append(listHash, articleHash)
				fmt.Printf("Hash: %d, Time: %v\n\n", articleHash.Hash, value.Pubdate)
			}
		}
	}

	// "Старые" статьи удаляем из списка хешей
	// Сортируем так, что более старые сначала
	if len(listHash) > 0 {
		sort.Slice(listHash, func(i, j int) bool { return listHash[i].Created.Unix() < listHash[j].Created.Unix() })
		i := 0
		for _, value := range listHash {
			// Прерываем цикл, когда старые закончились
			d = time.Since(value.Created)
			if d.Seconds() < newsage {
				break
			}
			i += 1
		}
		if i > 0 {
			listHash = listHash[i:]
		}
	}

	file, err := json.MarshalIndent(listHash, "", " ")
	if err != nil {
		fmt.Printf("Error - %v\n", err)
	}
	err = os.WriteFile(path+"/hashes.json", file, 0644)
	if err != nil {
		fmt.Printf("Error write Article hashes - %v\n", err)
	}
}

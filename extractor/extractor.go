package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	// _ "github.com/mattn/go-sqlite3"
	_ "github.com/go-sql-driver/mysql"
	"github.com/streadway/amqp"
	// amqp "github.com/rabbitmq/amqp091-go"
)

type Page struct {
	Id              int
	Url             string
	Last_crawl_time sql.NullInt64
	Crawl_count     sql.NullInt64
	File            sql.NullString
	New_pages       sql.NullInt64
	Found_from      int
}

func main() {
	amqp_conn, err := amqp.Dial("amqp://user:password@rabbitmq:5672/")
	checkErr("AMQP Connection", err)
	defer amqp_conn.Close()

	ch, err := amqp_conn.Channel()
	checkErr("AMQP Channel", err)
	defer ch.Close()

	msgs, err := ch.Consume(
		"extractor",
		"",
		false, //auto-ack
		false,
		false,
		false,
		nil,
	)
	checkErr("AMQP Consume", err)

	// db, err := sql.Open("sqlite3", "../database/data.db")
	db, err := sql.Open("mysql", "root:password@tcp(db:3306)/crawler")
	checkErr("Database Open", err)
	defer db.Close()

	fmt.Println("Successfully Connected to our RabbitMQ Instance")
	fmt.Println(" [*] - Waiting for messages")

	var wg sync.WaitGroup

	//limit number of go routine
	maxGoroutines := 50
	guard := make(chan struct{}, maxGoroutines)

	for d := range msgs {
		var pg Page
		err := json.Unmarshal(d.Body, &pg)
		checkErr("JSON Unmarshal", err)
		wg.Add(1)

		//limit number of go routine
		guard <- struct{}{} // would block if guard channel is already filled

		go func(delivery amqp.Delivery, pg Page) {
			defer wg.Done()
			defer func() {
				<-guard
			}()
			links, err := worker(pg)
			// fmt.Println("Got: " + string(len(links)) + " from " + pg.Url)
			checkErr("Extract Links", err)

			//update the crawled page
			stmt, err := db.Prepare("update pages set last_crawl_time=?, crawl_count=?, file=?, new_pages=? where url=?")
			checkErr("update statement prepare", err)
			defer stmt.Close()

			updated_crawl_count := pg.Crawl_count.Int64 + 1

			res, err := stmt.Exec(time.Now().UnixNano()/int64(time.Millisecond), updated_crawl_count, pg.File.String, len(links), pg.Url)
			checkErr("update statement exec", err)

			// affect, err := res.RowsAffected()
			_, err = res.RowsAffected()
			checkErr("rows affected", err)

			// fmt.Println("Updated crawled page: ", affect)

			//filter only new links
			/////////////////////////////////////////////////////////////////
			//     TODO : optiminze this section                           //
			/////////////////////////////////////////////////////////////////

			rows, err := db.Query("SELECT url FROM `pages`")
			checkErr("Database Query", err)
			defer rows.Close()

			var urls []string

			for rows.Next() {
				var s string
				rows.Scan(&s)
				urls = append(urls, s)
			}

			//add new links
		outer:
			for _, link := range links {
				for _, l := range urls {
					if link == l {
						continue outer
					}
				}

				// insert
				stmt, err := db.Prepare("INSERT IGNORE INTO `pages` (url, crawl_count, found_from) values(?,?,?)")
				checkErr("Inset new link statement prep", err)
				defer stmt.Close()

				res, err := stmt.Exec(link, 0, pg.Id)
				checkErr("insert link statement exec", err)

				// id, err := res.LastInsertId()
				_, err = res.LastInsertId()
				checkErr("insert new link id", err)

				// fmt.Println("New page inserted with id", id)
			}

			// fmt.Println("DONE: ", pg.Url)
			delivery.Ack(false)
		}(d, pg)
	}

	wg.Wait()
}

func worker(pg Page) ([]string, error) {
	f := pg.File.String
	html, err := readHtmlFromFile("/data/" + f)
	if err != nil {
		checkErr("Read from file", err)
		fmt.Println(pg.Id)
		fmt.Println("------------------------------------")
	}
	links := extractLinks(html)
	return links, nil
}

func readHtmlFromFile(fileName string) ([]byte, error) {
	bs, err := ioutil.ReadFile(fileName)
	if err != nil {
		return []byte(""), err
	}
	return bs, nil
}

func extractLinks(b []byte) []string {
	var links []string

	z := html.NewTokenizer(bytes.NewReader(b))

outer:
	for {
		tt := z.Next()

		switch {
		case tt == html.ErrorToken:
			// End of the document, we're done
			break outer
		case tt == html.StartTagToken:
			t := z.Token()

			// Check if the token is an <a> tag
			isAnchor := t.Data == "a"
			if !isAnchor {
				continue
			}

			// Extract the href value, if there is one
			ok, url := getHref(t)
			if !ok {
				continue
			}

			// Make sure the url begines in http**
			hasProto := strings.Index(url, "http") == 0
			if hasProto {
				//ch <- url
				links = append(links, url)
			}
		}
	}
	return links
}

func getHref(t html.Token) (ok bool, href string) {
	// Iterate over token attributes until we find an "href"
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	// "bare" return will return the variables (ok, href) as
	// defined in the function definition
	return
}

func checkErr(txt string, err error) {
	red := string("\033[0;31m")
	reset := string("\033[0m")
	if err != nil {
		fmt.Println(red, txt+": "+err.Error(), reset)
	}
}

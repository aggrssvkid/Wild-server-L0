package main

import (
	"log"
	"encoding/json"
	"database/sql"
	"net/http"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/nats-io/stan.go"
	_ "github.com/lib/pq"
)

//json структурка, в которой будут храниться данные одного объекта
type book struct {
	Id      string	`json:"id"`
	Title   string	`json:"title"`
	Author	string	`json:"author"`
	Price	string	`json:"price"`
  }

var	db		*sql.DB
// в cache сохраняем всю базу данных
var cache	map[string]book

func main () {
	var err	error

	connStr := "postgres://me:me@localhost:5432/me"
	db, err = sql.Open("postgres", connStr)

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
        log.Fatal(err)
    }

	cache = make(map[string]book)
	get_cache(db)

	//подрубаемся к nats-streaming
	sc, err := stan.Connect("test-cluster", "subscribe-id")
	if err != nil {
		log.Fatalln(err)
	}
	defer sc.Close()

	//подписываемся на канал
	sub, err := sc.Subscribe("book", func(m *stan.Msg) {

		var data book
		//раскладываем считанные данные в соответствующие поля по структурке book
		err = json.Unmarshal(m.Data, &data)
		if err != nil {
			log.Printf("New book not added: %s", err.Error())
			return
		}

		fmt.Println(data)
		if data.Id == "" {
			log.Printf("New order not added: empty Id")
			return
		}
		// добавляем книжечку в базу данных
		_, err = db.Exec("INSERT INTO books VALUES($1, $2, $3, $4)", data.Id, data.Title, data.Author, data.Price)
		if err != nil {
			log.Printf("DB execution error, %s", err.Error())
			return
		}
		cache[data.Id] = data
		log.Printf("book added!")
	})
	defer sub.Close()

	router := mux.NewRouter()
	router.HandleFunc("/", home)
	router.HandleFunc("/books", show_all_books)
	router.HandleFunc("/books/{id}", get_book)
	err = http.ListenAndServe(":3000", router)

	if err != nil {
		log.Fatal(err)
	}
}

func	get_cache(db *sql.DB) {
	rows, err := db.Query("select * from books")
	if err != nil {
		log.Fatalln(err)
	}

	defer rows.Close()

	for rows.Next() {
		bk := book{}
		err = rows.Scan(&bk.Id, &bk.Title, &bk.Author, &bk.Price)
		if err != nil {
			log.Fatal(err)
		}
		cache[bk.Id] = bk
	}

	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
}

//далее идут функции-обработчики запросов

func	home(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintf(w, "Zdarova, Otec!")
}

func	show_all_books(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
        http.Error(w, http.StatusText(405), 405)
        return
    }

	for _, value := range cache {
		jn, err := json.Marshal(value)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(w, "%s\n", jn)
	}
}

func	get_book(w http.ResponseWriter, r *http.Request) {
    params := mux.Vars(r)
    for key, value := range cache {
       if key == params["id"] {
        	js, err := json.Marshal(value)
			if err != nil {
			log.Println(err.Error())
			http.Error(w, "Internal Server Error", 500)
		}
			fmt.Fprintf(w, "%s", js)
        	return
        }
    }
	fmt.Fprintf(w, "No book with this id!")
}
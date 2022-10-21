package main

import (
	"database/sql" // 실제 SQL을 수행할 패키지
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gosimple/slug"

	_ "github.com/go-sql-driver/mysql" // driver는 개발자가 직접 쓰지 않으므로, 언더바로 alias를 준다
)

var db *sql.DB // 전역변수로 설정 - 아직 더 나은 대안은 모르겠다..

func main() {
	var err error
	db, err = sql.Open("mysql", "dylan:mysql@tcp(172.17.0.4:3306)/go_server?parseTime=true") // mysql is on another container, in same network "bridge"
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()

	// write any response with http.ResponseWriter
	// handle any request with http.Request
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Hello World"))
		fmt.Println(req.URL)
	})

	// 또 다른 HTTP handler 정의 방식 : http.Handle() 사용
	// 두 번째 파라미터에 http.Handler 객체를 받음
	http.Handle("/api/articles/", new(postHandler))

	// In Go, nil is the zero value for pointers, interfaces, maps, slices, channels and function types, representing an uninitialized value.
	// second arg: ServeMux. if nil, DefaultServeMux is set
	http.ListenAndServe(":8080", nil)
}

type postHandler struct {
	http.Handler
}

// `json:something`와 같은 태깅으로 go에게 json을 인코딩/디코딩하는 방법을 알려준다
// struct에서는 반드시 대문자로 시작한다(그렇지 않으면 export 안 됨)
type Article struct {
	Title   string   `json:"title"`
	Desc    string   `json:"description"`
	Body    string   `json:"body"`
	TagList []string `json:"tagList,omitempty"` //omitempty 태그로 옵션화
}
type ArticleOptions struct {
	Title string `json:"title,omitempty"`
	Desc  string `json:"description,omitempty"`
	Body  string `json:"body,omitempty"`
}
type Author struct {
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	Image     string `json:"image"`
	Following bool   `json:"following"`
}

// go에는 "상속"개념은 없지만 동일한 역할을 하는 "Embedding"이 있다
type ArticleRes struct {
	Article
	Slug           string    `json:"slug"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Favorited      bool      `json:"favorited"`
	FavoritesCount int       `json:"favoritesCount"`
	Author         Author    `json:"author"`
}
type ArticleRequestData struct {
	Article Article `json:"article"`
}
type ArticleUpdateData struct {
	Article ArticleOptions `json:"article"`
}
type ArticleResponseData struct {
	Article ArticleRes `json:"article"`
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func Create(db *sql.DB) {}

// TODO : response 하드코딩 목록 수정
func (h *postHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Method : ", req.Method)

	// 주의: Body는 ReadCloser객체이므로, 두번 읽으면 사라지게 된다(EOF만 읽힘)
	// body, _ := ioutil.ReadAll((req.Body))
	// fmt.Println("Body : ", string(body))
	var err error
	var reqData ArticleRequestData
	var req_slug string
	// var optionCheckMap map[string]interface{}

	if req.Method == "POST" {
		err = json.NewDecoder(req.Body).Decode(&reqData)
		defer req.Body.Close() // defer: 지연실행. 함수 종료 직전에 수행한다.
		if err != nil {
			fmt.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// // POST에서는 옵션이 아닌 것들을 점검한다.
		// // 이런 방법을 쓰기보단 method별로 기능을 구분해야할 것 같다.
		// temp_data, _ := json.Marshal(reqData.Article)
		// json.Unmarshal(temp_data, &optionCheckMap)
		// _, ok := optionCheckMap["Desc"]
		// if ok == false {
		// 	http.Error(w, err.Error(), http.StatusBadRequest)
		// 	return
		// }
		// _, ok = optionCheckMap["Body"]
		// if ok == false {
		// 	http.Error(w, err.Error(), http.StatusBadRequest)
		// 	return
		// }
		req_slug = slug.Make(reqData.Article.Title)
	} else {
		req_slug = strings.TrimPrefix(req.URL.Path, "/api/articles/")
		if len(req_slug) == 0 {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// user 쿼리
	query := fmt.Sprintf("SELECT * FROM realworld_user WHERE username='%s'", "jake")
	var user_id int
	var author Author
	err = db.QueryRow(query).Scan(&user_id, &author.Username, &author.Bio, &author.Image, &author.Following)
	checkError(err)

	// article 쿼리
	query = fmt.Sprintf("SELECT * FROM article WHERE slug='%s'", req_slug)
	var article_id int
	var article ArticleRes
	var temp_taglist string
	var temp_user_id int
	article_err := db.QueryRow(query).Scan(&article_id, &article.Slug, &article.Title, &article.Desc, &article.Body, &temp_taglist, &article.CreatedAt, &article.UpdatedAt, &article.Favorited, &article.FavoritesCount, &temp_user_id)

	switch req.Method {
	case "POST":

		// article이 없다면 INSERT
		if article_err == sql.ErrNoRows {
			article.CreatedAt = time.Now()
			article.UpdatedAt = article.CreatedAt
			_, err = db.Exec("INSERT INTO article VALUES (?,?,?,?,?,?,?,?,?,?,?)",
				nil,
				req_slug,
				reqData.Article.Title,
				reqData.Article.Desc,
				reqData.Article.Body,
				"["+strings.Join(reqData.Article.TagList, ",")+"]",
				article.CreatedAt,
				article.UpdatedAt,
				false, // favorited
				0,     // favoritescount
				user_id,
			)
			checkError(err)
		} else {
			checkError(article_err)
			article.UpdatedAt = time.Now()
		}

		// define response data
		resData := &ArticleResponseData{
			Article: ArticleRes{
				Article:        reqData.Article,
				Slug:           req_slug,
				CreatedAt:      article.CreatedAt,
				UpdatedAt:      article.UpdatedAt,
				Favorited:      false,
				FavoritesCount: 0,
				Author:         author,
			},
		}

		json.NewEncoder(w).Encode(resData)

	case "GET":
		checkError(article_err)
		article.Author = author
		article.TagList = strings.Split(temp_taglist, ",")
		resData := &ArticleResponseData{Article: article}
		json.NewEncoder(w).Encode(resData)
		w.Write([]byte("single article"))

	// TODO: optional fields required
	case "PUT":
		var reqUpdateData ArticleUpdateData
		err = json.NewDecoder(req.Body).Decode(&reqUpdateData)
		defer req.Body.Close() // defer: 지연실행. 함수 종료 직전에 수행한다.
		if err != nil {
			fmt.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		update_query := "UPDATE article SET title=?, description=?, body=?, slug=? WHERE id=?"
		_, err = db.Exec(update_query, reqUpdateData.Article.Title, reqUpdateData.Article.Desc, reqUpdateData.Article.Body, req_slug, article_id)
		checkError(err)
	case "DELETE":
		_, err = db.Exec("DELETE FROM article WHERE id=?", article_id)
		checkError(err)
	}
}

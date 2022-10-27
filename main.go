package main

import (
	"database/sql" // 실제 SQL을 수행할 패키지
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/gosimple/slug"

	_ "github.com/go-sql-driver/mysql" // driver는 개발자가 직접 쓰지 않으므로, 언더바로 alias를 준다
)

var db *sql.DB // 전역변수로 설정 - 아직 더 나은 대안은 모르겠다..

func main() {
	var err error
	db, err = sql.Open("mysql", "dylan:mysql@tcp(172.17.0.4:3306)/go_server?parseTime=true") // mysql is on another container, in same network "bridge"
	if err != nil || db.Ping() != nil {
		fmt.Println(err)
	}
	defer db.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/articles", handleArticle).Methods("POST")
	r.HandleFunc("/api/articles/{slug}", handleArticle).Methods("GET")
	r.HandleFunc("/api/articles/{slug}", handleArticle).Methods("PUT")
	r.HandleFunc("/api/articles/{slug}", handleArticle).Methods("DELETE")
	r.HandleFunc("/api/articles/{slug}/comments", handleComment).Methods("POST")
	r.HandleFunc("/api/articles/{slug}/comments", handleComment).Methods("GET")
	r.HandleFunc("/api/articles/{slug}/comments/{id}", handleComment).Methods("DELETE")

	server := &http.Server{Addr: ":8080", Handler: r}
	server.ListenAndServe()
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

type Comment struct {
	Id        int       `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    Author    `json:"author"`
}
type CommentList struct {
	Comments []Comment `json:"comments"`
}
type CommentRequest struct {
	Body string `json:"body"`
}
type CommentRequestData struct {
	Comment CommentRequest `json:"comment"`
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func handleArticle(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Method : ", req.Method)

	// 주의: Body는 ReadCloser객체이므로, 두번 읽으면 사라지게 된다(EOF만 읽힘)
	// body, _ := ioutil.ReadAll((req.Body))
	// fmt.Println("Body : ", string(body))
	var err error
	var reqData ArticleRequestData
	var req_slug string

	if req.Method == "POST" {
		err = json.NewDecoder(req.Body).Decode(&reqData)
		checkError(err)
		defer req.Body.Close() // defer: 지연실행. 함수 종료 직전에 수행한다.
		if err != nil {
			fmt.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req_slug = slug.Make(reqData.Article.Title)
	} else {
		// req_slug = strings.TrimPrefix(req.URL.Path, "/api/articles/")
		vars := mux.Vars(req)
		req_slug = vars["slug"]
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
		// w.Write([]byte("single article"))

	// TODO: optional fields required
	case "PUT":
		var reqUpdateData ArticleUpdateData
		err = json.NewDecoder(req.Body).Decode(&reqUpdateData)
		defer req.Body.Close()
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

func handleComment(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Method : ", req.Method)
	vars := mux.Vars(req)
	req_slug := vars["slug"]

	// slug로 article 쿼리
	// 복수의 rows를 받기 위해 Query()와 Next()를 사용
	query := fmt.Sprintf("SELECT * FROM article WHERE slug='%s'", req_slug)
	var article_id int
	var article ArticleRes
	var temp_taglist string
	var temp_user_id int
	err := db.QueryRow(query).Scan(&article_id, &article.Slug, &article.Title, &article.Desc, &article.Body, &temp_taglist, &article.CreatedAt, &article.UpdatedAt, &article.Favorited, &article.FavoritesCount, &temp_user_id)
	checkError(err)

	// article로 user 쿼리
	query = fmt.Sprintf("SELECT * FROM realworld_user WHERE id=%d", temp_user_id)
	var user_id int
	var author Author
	err = db.QueryRow(query).Scan(&user_id, &author.Username, &author.Bio, &author.Image, &author.Following)
	checkError(err)

	switch req.Method {
	case "POST":
		var reqData CommentRequestData
		var resData Comment
		err = json.NewDecoder(req.Body).Decode(&reqData)
		checkError(err)
		defer req.Body.Close()

		resData.Id = article_id
		resData.CreatedAt = time.Now()
		resData.UpdatedAt = resData.CreatedAt // POST로 생성시에는 create time과 같다
		resData.Body = reqData.Comment.Body
		resData.Author = author
		_, err = db.Exec("INSERT INTO comment VALUES (?,?,?,?,?,?)",
			nil,
			resData.CreatedAt,
			resData.UpdatedAt,
			resData.Body,
			temp_user_id,
			article_id,
		)
		checkError(err)
		json.NewEncoder(w).Encode(resData)

	case "GET":
		var resData CommentList

		// comment 쿼리
		query = fmt.Sprintf("SELECT id, createdAt, updatedAt, body FROM comment WHERE article_id=%d", article_id)
		rows, err := db.Query(query)
		checkError(err)
		defer rows.Close()

		// query를 iterate하면서 comments를 받아놓음
		for rows.Next() {
			var comment Comment
			comment.Author = author
			err := rows.Scan(&comment.Id, &comment.CreatedAt, &comment.UpdatedAt, &comment.Body)
			checkError(err)
			resData.Comments = append(resData.Comments, comment)
		}

		json.NewEncoder(w).Encode(resData)

	case "DELETE":
		comment_id := vars["id"]
		_, err = db.Exec("DELETE FROM comment WHERE id=?", comment_id)
		checkError(err)

		w.Write([]byte("delete completed"))
	}
}

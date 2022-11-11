package main

import (
	"database/sql" // 실제 SQL을 수행할 패키지
	"encoding/json"

	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/gosimple/slug"

	"github.com/go-sql-driver/mysql" // driver는 개발자가 직접 쓰지 않으므로, 언더바로 alias를 준다
)

var db *sql.DB // 전역변수로 설정 - 아직 더 나은 대안은 모르겠다..
var mysqlErr *mysql.MySQLError

// 전체 TODO: author following 개선
func main() {
	var err error
	db, err = sql.Open("mysql", "dylan:mysql@tcp(172.17.0.4:3306)/go_server?parseTime=true") // mysql is on another container, in same network "bridge"
	if err != nil || db.Ping() != nil {
		fmt.Println(err)
	}
	defer db.Close()

	r := mux.NewRouter()
	r.HandleFunc("/api/articles", handleArticle).Methods("POST")
	r.HandleFunc("/api/articles", handleListArticle).Methods("GET")
	r.HandleFunc("/api/articles/{slug}", handleArticle).Methods("GET")
	r.HandleFunc("/api/articles/{slug}", handleArticle).Methods("PUT")
	r.HandleFunc("/api/articles/{slug}", handleArticle).Methods("DELETE")

	r.HandleFunc("/api/articles/{slug}/favorite", handleFavorite).Methods("POST")
	r.HandleFunc("/api/articles/{slug}/favorite", handleFavorite).Methods("DELETE")

	r.HandleFunc("/api/articles/{slug}/comments", handleComment).Methods("POST")
	r.HandleFunc("/api/articles/{slug}/comments", handleComment).Methods("GET")
	r.HandleFunc("/api/articles/{slug}/comments/{id}", handleComment).Methods("DELETE")

	r.HandleFunc("/api/tags", handleTag).Methods("GET")

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
type MultiArticleResponseData struct {
	Articles []ArticleRes `json:"articles"`
	Counts   int          `json:"articlesCount"`
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

type TagResponseData struct {
	Tags []string `json:"tags"`
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
	var temp_user_id int
	article_err := db.QueryRow(query).Scan(&article_id, &article.Slug, &article.Title, &article.Desc, &article.Body, &article.CreatedAt, &article.UpdatedAt, &temp_user_id)

	switch req.Method {
	case "POST":

		// TODO: 아래 article이 없을때의 조건을 바꾸자.(테이블의 slug가 unique조건임을 활용)
		// article이 없다면 INSERT
		if article_err == sql.ErrNoRows {
			article.CreatedAt = time.Now()
			article.UpdatedAt = article.CreatedAt
			_, err = db.Exec("INSERT INTO article VALUES (?,?,?,?,?,?,?,?)",
				nil,
				req_slug,
				reqData.Article.Title,
				reqData.Article.Desc,
				reqData.Article.Body,
				article.CreatedAt,
				article.UpdatedAt,
				user_id,
			)
			checkError(err)

			// 추가된 article의 id를 조회
			query = fmt.Sprintf("SELECT id FROM article WHERE slug='%s'", req_slug)
			err = db.QueryRow(query).Scan(&article_id)
			checkError(err)

			// INSERT tag
			for _, tag_name := range reqData.Article.TagList {
				// tag에 신규 태그 추가 : IGNORE를 줘서 이미 존재하는 태그라면 DUPLICATED KEY 에러를 무시한다.
				_, err = db.Exec("INSERT IGNORE INTO tag VALUES (?,?)",
					nil,
					tag_name,
				)
				checkError(err)

				// (deprecated : 잘못된 로직) tag 테이블에 중복 발생 시, article_tag에 INSERT 작업은 무시한다.
				// var mysqlErr *mysql.MySQLError
				// if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 { continue }

				// tag의 id를 다시 조회
				query = fmt.Sprintf("SELECT id FROM tag WHERE tag_name='%s'", tag_name)
				var tag_id int
				err = db.QueryRow(query).Scan(&tag_id)
				checkError(err)

				// article_tag 테이블에 관계 추가
				_, err = db.Exec("INSERT INTO article_tag VALUES (?,?,?)",
					nil,
					tag_id,
					article_id,
				)
				checkError(err)
			}

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

		// tag 쿼리
		tagList := getTags(article_id)

		// TODO: Authentication 추가 후, 실제 user_id 입력
		auth_user_id := 1
		// user_id, article_id로 favorite 쿼리
		query = fmt.Sprintf("SELECT user_id FROM favorite WHERE article_id='%d'", article_id)
		var favorites_cnt int = 0
		var temp_user_id int
		var favorited bool = false
		rows, err := db.Query(query)
		defer rows.Close()
		for rows.Next() {
			err := rows.Scan(&temp_user_id)
			if temp_user_id == auth_user_id {
				favorited = true
			}
			checkError(err)
			favorites_cnt += 1
		}
		checkError(err)

		article.Author = author
		article.TagList = tagList
		article.Favorited = favorited
		article.FavoritesCount = favorites_cnt
		resData := &ArticleResponseData{Article: article}
		json.NewEncoder(w).Encode(resData)
		// w.Write([]byte("single article"))

	// TODO: optional fields required
	// TODO: tag 업데이트
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
		// article_tag에서 tag 관계도 삭제
		_, err = db.Exec("DELETE FROM article_tag WHERE article_id=?", article_id)
		checkError(err)

		// favorite에서 관계도 삭제
		_, err = db.Exec("DELETE FROM favorite WHERE article_id=?", article_id)
		checkError(err)

		// comment 삭제
		_, err = db.Exec("DELETE FROM comment WHERE article_id=?", article_id)
		checkError(err)

		// article 삭제
		_, err = db.Exec("DELETE FROM article WHERE id=?", article_id)
		checkError(err)
	}
}

// TODO: 일단은 종류별로 하나의 파라미터만 받는다고 가정한다..
// TODO: option 조건에 만족하지 않을 때 Error Handling 없음.
// TODO: 쿼리 string에 모든걸 떠안는 tag, favorite 필터에 대해 JOIN 방식으로 대체해보기..
// TODO: 최종 결과물인 article SELECT 쿼리 또한 결과물에 Author정보(realworld_user 테이블) JOIN해서 가져오기..
func handleListArticle(w http.ResponseWriter, req *http.Request) {
	var limit int64 = 20
	var offset int64 = 0
	var err error
	// Query parameters 받기
	tag := req.FormValue("tag")             // filter by tag
	author_name := req.FormValue("author")  // filter by author
	favorited := req.FormValue("favorited") // favorited artices by this user
	limit_str := req.FormValue("limit")     // limit number
	if limit_str != "" {
		limit, err = strconv.ParseInt(limit_str, 10, 32) // 10진수, 32bit
		checkError(err)
	}
	offset_str := req.FormValue("offset") // offset
	if offset_str != "" {
		offset, err = strconv.ParseInt(offset_str, 10, 32) // 10진수, 32bit
		checkError(err)
	}

	// article 쿼리 준비
	query := "SELECT * FROM article"
	var conditions string
	if tag != "" || author_name != "" || favorited != "" {
		query = query + " WHERE"
		conditions = ""
	}
	// tag 필터링
	if tag != "" {
		tag_query := fmt.Sprintf("SELECT article_id FROM article_tag WHERE tag_id=(SELECT id FROM tag WHERE tag_name='%s')", tag)
		tag_rows, err := db.Query(tag_query)
		checkError(err)
		defer tag_rows.Close()
		var article_ids []string
		var article_id string
		for tag_rows.Next() {
			err := tag_rows.Scan(&article_id)
			checkError(err)
			article_ids = append(article_ids, fmt.Sprintf("%q", article_id))
		}
		if len(article_ids) > 0 {
			ids_str := strings.Join(article_ids, ",")
			if conditions != "" {
				conditions = conditions + " AND"
			}
			conditions = conditions + fmt.Sprintf(" id in (%s)", ids_str)
		}
	}
	// favorited user 필터링
	if favorited != "" {
		fav_query := fmt.Sprintf("SELECT article_id FROM favorite WHERE user_id=(SELECT id FROM realworld_user WHERE username='%s')", favorited)
		fav_rows, err := db.Query(fav_query)
		checkError(err)
		defer fav_rows.Close()
		var article_ids []string
		var article_id string
		for fav_rows.Next() {
			err := fav_rows.Scan(&article_id)
			checkError(err)
			article_ids = append(article_ids, fmt.Sprintf("%q", article_id))
		}
		if len(article_ids) > 0 {
			ids_str := strings.Join(article_ids, ",")
			if conditions != "" {
				conditions = conditions + " AND"
			}
			conditions = conditions + fmt.Sprintf(" id in (%s)", ids_str)
		}
	}
	// author 필터링
	if author_name != "" {
		author_query := fmt.Sprintf("SELECT id FROM realworld_user WHERE username='%s'", author_name)
		var author_id int
		err := db.QueryRow(author_query).Scan(&author_id)
		checkError(err)
		if conditions != "" {
			conditions = conditions + " AND"
		}
		conditions = conditions + fmt.Sprintf(" user_id=%d", author_id)
	}
	if conditions != "" {
		query = query + conditions
	}
	query = query + fmt.Sprintf(" ORDER BY createdAt DESC LIMIT %d OFFSET %d", limit, offset)
	rows, err := db.Query(query)
	checkError(err)
	defer rows.Close()
	var resData MultiArticleResponseData
	article_cnt := 0
	for rows.Next() {
		var article ArticleRes
		var article_id int
		var author_id int
		err = rows.Scan(&article_id, &article.Slug, &article.Title, &article.Desc, &article.Body, &article.CreatedAt, &article.UpdatedAt, &author_id)
		checkError(err)

		// get Tag
		tag_query := fmt.Sprintf("SELECT tag_name FROM tag WHERE id in (SELECT tag_id FROM article_tag WHERE article_id=%d)", article_id)
		var tag_names []string
		var tag_name string
		tag_rows, err := db.Query(tag_query)
		checkError(err)
		for tag_rows.Next() {
			err = tag_rows.Scan(&tag_name)
			checkError(err)
			tag_names = append(tag_names, tag_name)
		}
		article.Article.TagList = tag_names

		// get Author
		author_query := fmt.Sprintf("SELECT * FROM realworld_user WHERE id=%d", author_id)
		err = db.QueryRow(author_query).Scan(&author_id, &article.Author.Username, &article.Author.Bio, &article.Author.Image, &article.Author.Following)
		checkError(err)

		// final append
		resData.Articles = append(resData.Articles, article)
		article_cnt += 1
	}
	resData.Counts = article_cnt

	json.NewEncoder(w).Encode(resData)
}

func handleFavorite(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Method : ", req.Method)
	vars := mux.Vars(req)
	req_slug := vars["slug"]

	var query string
	var resData ArticleResponseData

	// article 쿼리
	query = fmt.Sprintf("SELECT * FROM article WHERE slug='%s'", req_slug)
	var article_id int
	var article ArticleRes
	var temp_author_id int
	err := db.QueryRow(query).Scan(&article_id, &article.Slug, &article.Title, &article.Desc, &article.Body, &article.CreatedAt, &article.UpdatedAt, &temp_author_id)
	checkError(err)

	// author 쿼리
	query = fmt.Sprintf("SELECT * FROM realworld_user WHERE id='%d'", temp_author_id)
	var author Author
	err = db.QueryRow(query).Scan(&temp_author_id, &author.Username, &author.Bio, &author.Image, &author.Following)
	checkError(err)

	// article_id로 favoritesCount 쿼리
	query = fmt.Sprintf("SELECT count(*) FROM favorite WHERE article_id='%d'", article_id)
	var favorites_cnt int
	err = db.QueryRow(query).Scan(&favorites_cnt)
	checkError(err)

	// response data 기본 정보 입력
	resData.Article = article
	resData.Article.Author = author

	// TODO: Authentication 추가 후, 실제 user_id 입력
	user_id := 1
	switch req.Method {
	case "POST":
		// 해당 user로 favorites 추가
		_, err = db.Exec("INSERT INTO favorite VALUES (?,?,?)",
			nil,
			user_id,
			article_id,
		)
		// DUPLICATED KEY error = 이미 favorite임
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		} else {
			favorites_cnt += 1
		}

		resData.Article.Favorited = true
		resData.Article.FavoritesCount = favorites_cnt
		json.NewEncoder(w).Encode(resData)

	case "DELETE":
		// user_id 및 article_id로 favoritesCount 쿼리
		query = fmt.Sprintf("SELECT count(*) FROM favorite WHERE user_id='%d' and article_id='%d'", user_id, article_id)
		var is_favorite int
		err = db.QueryRow(query).Scan(&is_favorite)
		checkError(err)

		// 해당 유저가 favorite 등록했다면 DELETE
		if is_favorite > 0 {
			_, err = db.Exec("DELETE FROM favorite WHERE user_id=? and article_id=?", user_id, article_id)
			checkError(err)
			favorites_cnt -= 1
		}

		resData.Article.Favorited = false
		resData.Article.FavoritesCount = favorites_cnt
		json.NewEncoder(w).Encode(resData)
	}

}

func handleComment(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Method : ", req.Method)
	vars := mux.Vars(req)
	req_slug := vars["slug"]

	// slug로 article 쿼리
	query := fmt.Sprintf("SELECT id, user_id FROM article WHERE slug='%s'", req_slug)
	var article_id int
	var temp_user_id int
	err := db.QueryRow(query).Scan(&article_id, &temp_user_id)
	checkError(err)

	switch req.Method {
	case "POST":
		var reqData CommentRequestData
		var resData Comment
		err = json.NewDecoder(req.Body).Decode(&reqData)
		checkError(err)
		defer req.Body.Close()

		// TODO: Authentication 추가 후, 실제 user_id 입력
		user_id := 1
		query = fmt.Sprintf("SELECT * FROM realworld_user WHERE id=%d", user_id)
		var author Author
		err = db.QueryRow(query).Scan(&user_id, &author.Username, &author.Bio, &author.Image, &author.Following)
		checkError(err)

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
		query = fmt.Sprintf("SELECT id, createdAt, updatedAt, body, user_id FROM comment WHERE article_id=%d", article_id)
		comment_rows, err := db.Query(query)
		checkError(err)
		defer comment_rows.Close()

		// users: comment마다 조회한 user를 저장해둠. 중복 user는 또다시 쿼리하지 않도록 캐시.
		users := make(map[int]Author) // map을 사용할때는 make로 초기화하지 않으면 runtime error 발생

		// query를 iterate하면서 comments를 받아놓음
		for comment_rows.Next() {
			var comment Comment
			var user_id int
			err := comment_rows.Scan(&comment.Id, &comment.CreatedAt, &comment.UpdatedAt, &comment.Body, &user_id)
			checkError(err)

			// user_id에 따라 user 탐색
			var author Author
			if author, ok := users[user_id]; !ok {
				query = fmt.Sprintf("SELECT * FROM realworld_user WHERE id=%d", user_id)
				err = db.QueryRow(query).Scan(&user_id, &author.Username, &author.Bio, &author.Image, &author.Following)
				checkError(err)
				users[user_id] = author
			}
			comment.Author = author

			resData.Comments = append(resData.Comments, comment)
		}

		json.NewEncoder(w).Encode(resData)

	case "DELETE":
		// TODO: Authentication 추가 후, user_id와 해당 comment가 일치하는지 먼저 확인 후 실행
		comment_id := vars["id"]
		_, err = db.Exec("DELETE FROM comment WHERE id=?", comment_id)
		checkError(err)

		w.Write([]byte("delete completed"))
	}
}

func handleTag(w http.ResponseWriter, req *http.Request) {
	// 전체 tag 쿼리
	tagList := getTags()

	var resData TagResponseData
	resData.Tags = tagList

	json.NewEncoder(w).Encode(resData)
}

func getTags(article_id ...int) []string {
	var query string = "SELECT tag_name FROM tag"

	if article_id != nil {
		temp_query := fmt.Sprintf("SELECT tag_id FROM article_tag WHERE article_id='%d'", article_id[0])
		temp_rows, err := db.Query(temp_query)
		checkError(err)
		defer temp_rows.Close()

		var tagIdList []string
		for temp_rows.Next() {
			var tag_id int
			err = temp_rows.Scan(&tag_id)
			checkError(err)
			tagIdList = append(tagIdList, strconv.Itoa(tag_id)) // int to string 변환이 특이하다..
		}
		if len(tagIdList) > 0 {
			tagIds := strings.Join(tagIdList, ",")

			query = fmt.Sprintf("SELECT tag_name FROM tag WHERE id in (%s)", tagIds)
		}
	}

	rows, err := db.Query(query)
	checkError(err)
	defer rows.Close()

	var tagList []string
	for rows.Next() {
		var tag_name string
		err = rows.Scan(&tag_name)
		checkError(err)
		tagList = append(tagList, tag_name)
	}

	return tagList
}

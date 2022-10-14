package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	// write any response with http.ResponseWriter
	// handle any request with http.Request
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Hello World"))
		fmt.Println(req.URL)
	})

	// 또 다른 HTTP handler 정의 방식 : http.Handle() 사용
	// 두 번째 파라미터에 http.Handler 객체를 받음
	http.Handle("/api/articles", new(postHandler))

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
type Author struct {
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	Image     string `json:"image"`
	Following bool   `json:"following"`
}

// go에는 "상속"개념은 없지만 동일한 역할을 하는 "Embedding"이 있다
type ArticleRes struct {
	Article
	Slug           string `json:"slug"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	Favorited      bool   `json:"favorited"`
	FavoritesCount int    `json:"favoritesCount"`
	Author         Author `json:"author"`
}
type ArticleRequestData struct {
	Article Article `json:"article"`
}
type ArticleResponseData struct {
	Article ArticleRes `json:"article"`
}

// TODO : response 하드코딩 목록 수정
func (h *postHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Method : ", req.Method)

	// 주의: Body는 ReadCloser객체이므로, 두번 읽으면 사라지게 된다(EOF만 읽힘)
	// body, _ := ioutil.ReadAll((req.Body))
	// fmt.Println("Body : ", string(body))

	reqData := new(ArticleRequestData)
	err := json.NewDecoder(req.Body).Decode(reqData)
	fmt.Println(reqData)
	defer req.Body.Close() // defer: 지연실행. 함수 종료 직전에 수행한다.
	if err != nil {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resData := &ArticleResponseData{
		Article: ArticleRes{
			Article:        reqData.Article,
			Slug:           "how-to-train-your-dragon",
			CreatedAt:      "2016-02-18T03:22:56.637Z",
			UpdatedAt:      "2016-02-18T03:48:35.827Z",
			Favorited:      false,
			FavoritesCount: 0,
			Author: Author{
				Username:  "jake",
				Bio:       "I work at statefarm",
				Image:     "https://i.stack.imgur.com/xHWG8.jpg",
				Following: false,
			},
		},
	}

	switch req.Method {
	case "POST":
		json.NewEncoder(w).Encode(resData)
	case "GET":
		w.Write([]byte("single article"))
	}
}

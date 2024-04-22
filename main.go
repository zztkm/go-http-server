package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-michi/michi"
	_ "github.com/mattn/go-sqlite3"

	"github.com/zztkm/go-http-server/gen/sqlc"
)

type todoHandler struct {
	db      *sql.DB
	querier *sqlc.Queries
}

func newTodoHandler(db *sql.DB) *todoHandler {
	return &todoHandler{db: db, querier: sqlc.New(db)}
}

func (h *todoHandler) index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, World!"))
}

type createTodoRequest struct {
	Title string `json:"title"`
}

func (h *todoHandler) createTodo(w http.ResponseWriter, r *http.Request) {
	var req createTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	todo, err := h.querier.CreateTodo(r.Context(), req.Title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(todo); err != nil {
		fmt.Fprintf(w, "Failed to encode response: %v", err)
	}
}

func (h *todoHandler) getTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	todo, err := h.querier.GetTodo(r.Context(), int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(todo); err != nil {
		fmt.Fprintf(w, "Failed to encode response: %v", err)
	}
}

func (h *todoHandler) listTodos(w http.ResponseWriter, r *http.Request) {
	todos, err := h.querier.ListTodos(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(todos) == 0 {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(todos); err != nil {
		fmt.Fprintf(w, "Failed to encode response: %v", err)
	}
	// TODO: あとで消す
	// Graceful Shutdown の確認用
	time.Sleep(5 * time.Second)

}

func main() {
	// TODO: Graceful Shutdown を完全に理解する
	// Graceful Shutdown 参考
	// https://shogo82148.github.io/blog/2023/09/21/2023-09-21-start-http-server/
	chSignal := make(chan os.Signal, 1)
	signal.Notify(chSignal, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(chSignal)

	db, err := sql.Open("sqlite3", "db.sqlite")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	h := newTodoHandler(db)

	r := michi.NewRouter()
	r.HandleFunc("/", h.index)
	r.HandleFunc("GET /todos", h.listTodos)
	r.HandleFunc("POST /todos", h.createTodo)
	r.Route("/todos", func(sub *michi.Router) {
		sub.HandleFunc("GET /{id}", h.getTodo)
	})
	http.Handle("/", r)

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	chServe := make(chan error, 1)
	go func() {
		defer close(chServe)
		chServe <- server.ListenAndServe()
	}()

	select {
	case err := <-chServe:
		log.Fatal(err)
	case <-chSignal:
	}

	signal.Stop(chSignal)

	// Graceful Shutdown 開始
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}

	// HTTPサーバーが完全に終了するのを待つ
	// (OSがリソース閉じてくれるはずだけど、お行儀よく）
	server.Close()
	<-chServe // http.ErrServerClosed が返ってくるのがわかっているので、特に何もしない。
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"

	jwt "github.com/golang-jwt/jwt/v4"
)

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")

	return json.NewEncoder(w).Encode(v)
}

type APIFunc func(http.ResponseWriter, *http.Request) error

type APIError struct {
	Error string
}

func makeHttpHandleFunc(f APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			writeJSON(w, http.StatusBadRequest, APIError{Error: err.Error()})
		}
	}
}

type APIServer struct {
	listenAddr string
	store      Storage
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()

	router.HandleFunc("/account", makeHttpHandleFunc(s.handleAccount))
	router.HandleFunc("/account/{id}", withJwtAuth(makeHttpHandleFunc(s.handleAccountById), s.store))
	router.HandleFunc("/transfer", makeHttpHandleFunc(s.handleTransfer))

	log.Println("JSON API server running on port: ", s.listenAddr)

	http.ListenAndServe(s.listenAddr, router)
}

func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetAccount(w, r)
	}

	if r.Method == "POST" {
		return s.handleCreateAccount(w, r)
	}

	if r.Method == "DELETE" {
		return s.handleDeleteAccount(w, r)
	}

	return fmt.Errorf("method not allowed %s", r.Method)
}

func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	accounts, err := s.store.GetAccounts()

	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, accounts)
}

func (s *APIServer) handleAccountById(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetAccountById(w, r)
	}

	if r.Method == "PUT" {
		return s.handleUpdateAccount(w, r)
	}

	if r.Method == "DELETE" {
		return s.handleDeleteAccount(w, r)
	}

	return fmt.Errorf("method not allowed %s", r.Method)
}

func (s *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	createAccountRequest := new(AccountRequest)

	if err := json.NewDecoder(r.Body).Decode(createAccountRequest); err != nil {
		return err
	}

	account := NewAccount(createAccountRequest.FirstName, createAccountRequest.LastName)

	if err := s.store.CreateAccount(account); err != nil {
		return err
	}

	tokenString, err := createJwt(account)

	if err != nil {
		return err
	}

	fmt.Println("Json Web Token: ", tokenString)

	return writeJSON(w, http.StatusOK, account)
}

func createJwt(account *Account) (string, error) {
	claims := &jwt.MapClaims{
		"expiresAt":     15000,
		"accountNumber": account.Number,
	}

	secret := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte((secret)))
}

func (s *APIServer) handleUpdateAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := getIdFromQueryParams(r)

	if err != nil {
		return fmt.Errorf("invalid id given %d", id)
	}

	account, err := s.store.GetAccountById(id)

	if err != nil {
		return err
	}

	accountRequest := new(AccountRequest)

	if err := json.NewDecoder(r.Body).Decode(accountRequest); err != nil {
		return err
	}

	account.FirstName = accountRequest.FirstName
	account.LastName = accountRequest.LastName

	if err := s.store.UpdateAccount(account); err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleGetAccountById(w http.ResponseWriter, r *http.Request) error {
	id, err := getIdFromQueryParams(r)

	if err != nil {
		return fmt.Errorf("invalid id given %d", id)
	}

	account, err := s.store.GetAccountById(id)

	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := getIdFromQueryParams(r)

	if err != nil {
		return fmt.Errorf("invalid id given %d", id)
	}

	if err := s.store.DeleteAccount(id); err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, id)
}

func (s *APIServer) handleTransfer(w http.ResponseWriter, r *http.Request) error {

	transferRequest := new(TransferRequest)

	if err := json.NewDecoder(r.Body).Decode(transferRequest); err != nil {
		return err
	}

	defer r.Body.Close()

	return writeJSON(w, http.StatusOK, transferRequest)
}

func validateJwt(tokenString string) (*jwt.Token, error) {
	jwtSecret := os.Getenv("JWT_SECRET")

	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return []byte(jwtSecret), nil
	})
}

func withJwtAuth(handleFunc http.HandlerFunc, s Storage) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Calling method with jwt authentication")

		tokenString := r.Header.Get("x-jwt-token")

		token, err := validateJwt(tokenString)

		if err != nil || !token.Valid {
			writeJSON(w, http.StatusForbidden, APIError{Error: "Permission denied"})
			return
		}

		if !token.Valid {
			writeJSON(w, http.StatusForbidden, APIError{Error: "Invalid token"})
			return
		}

		userId, err := getIdFromQueryParams(r)

		if err != nil {
			writeJSON(w, http.StatusForbidden, APIError{Error: "Invalid token"})
			return
		}

		account, err := s.GetAccountById(userId)

		if err != nil {
			writeJSON(w, http.StatusForbidden, APIError{Error: "Invalid token"})
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		if account.Number != int64(claims["accountNumber"].(float64)) {
			writeJSON(w, http.StatusForbidden, APIError{Error: "Invalid token"})
			return
		}

		if err != nil {
			writeJSON(w, http.StatusForbidden, APIError{Error: "Invalid token"})
			return
		}

		handleFunc(w, r)
	}

}

func getIdFromQueryParams(r *http.Request) (int, error) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)

	if err != nil {
		return -1, err
	}

	return id, nil
}

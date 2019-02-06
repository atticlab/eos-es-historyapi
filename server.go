package main

import (
	"fmt"
	"time"
	"sync"
	"io/ioutil"
	"net/http"
	"encoding/json"
	"github.com/olivere/elastic"
)


const ApiPath                      string = "/v1/history/"
const AccountsIndexPrefix          string = "accounts"
const TransactionsIndexPrefix      string = "transactions"
const TransactionTracesIndexPrefix string = "transaction_traces"
const ActionTracesIndexPrefix      string = "action_traces"
const FetchIndexListIntervalSeconds int64 = 30


type Config struct {
	Port       uint32 `json:"port"`
	ElasticUrl string `json:"elastic_url"`
}


type Server struct {
	ElasticUrl string
    ElasticClient *elastic.Client
	Indices map[string][]string
	//syncronization for Indices
	Wg1 sync.WaitGroup
	Wg2 sync.WaitGroup
}


func (s * Server) listen(port uint32) {
	err := http.ListenAndServe(":" + fmt.Sprint(port), nil)
    if err != nil {
        panic(err)
    }
}


func (s *Server) initElasticClient(url string) {
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetSniff(false))
	if err != nil {
		panic(err)
	} else {
		s.ElasticClient = client
		s.ElasticUrl = url
		go func () {
			for {
				s.fetchIndices()
				time.Sleep(time.Duration(FetchIndexListIntervalSeconds) * time.Second)
			}
		}()
	}
}

func (s *Server) setRoutes() {
	http.HandleFunc(ApiPath + "get_actions", s.onlyGetOrPost(s.handleGetActions()))
	http.HandleFunc(ApiPath + "get_transaction", s.onlyGetOrPost(s.handleGetTransaction()))
	http.HandleFunc(ApiPath + "get_key_accounts", s.onlyGetOrPost(s.handleGetKeyAccounts()))
	http.HandleFunc(ApiPath + "get_controlled_accounts", s.onlyGetOrPost(s.handleGetControlledAccounts()))
}


//onlyGet take function (http handler) as an argument
//and returns function that takes http.ResponseWriter and *http.Request
//this function will call given handler only if http method of the request is GET
//otherwise it will respond with 405 error code
func (s *Server) onlyGetOrPost(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if (r.Method != http.MethodGet && r.Method != http.MethodPost) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			response := ErrorResult { Code: http.StatusMethodNotAllowed, Message: "Invalid request method." }
			json.NewEncoder(w).Encode(response)
			return
		}
		h(w, r)
	}
}

func (s *Server) fetchIndices() {
	prefixes := []string {
		AccountsIndexPrefix,
		TransactionsIndexPrefix,
		TransactionTracesIndexPrefix,
		ActionTracesIndexPrefix }
	tmp := getIndices(s.ElasticUrl, prefixes)
	s.Wg1.Add(1)
	s.Wg2.Wait()
	s.Indices = tmp
	s.Wg1.Done()
}

func (s *Server) getIndices() map[string][]string {
	s.Wg1.Wait()
	s.Wg2.Add(1)
	result := s.Indices
	s.Wg2.Done()
	return result
}

//handleGetActions returns http handler that takes
//http.ResponseWriter and *http.Request as arguments
//it tries to parse parameters from request body
//and passes them to getActions()
//The result of getActions() is encoded and sent as a response
func (s *Server) handleGetActions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}

		var params GetActionsParams
		err = json.Unmarshal(bytes, &params)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResult { Code: http.StatusBadRequest, Message: "Invalid arguments." }
			json.NewEncoder(w).Encode(response)
			return
		}
		if params.Pos == nil {
			params.Pos = new(int64)
			*params.Pos = -1
		}
		if params.Offset == nil {
			params.Offset = new(int64)
			*params.Offset = -20
		}

		result, err := getActions(s.ElasticClient, params, s.getIndices())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}

		info, err := getInfo()
		if err == nil {
			result.LastIrreversibleBlock = info.LastIrreversibleBlockNum
		}

		b, err := json.Marshal(result)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}
		fmt.Fprintf(w, string(b))
	}
}

//handleGetTransaction returns http handler that takes
//http.ResponseWriter and *http.Request as arguments
//it tries to parse parameters from request body
//and passes them to getTransaction()
//retrieves block from node chain api
//and appends requested transaction info to getTransaction() result
//The result is encoded and sent as a response
func (s *Server) handleGetTransaction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}

		var params GetTransactionParams
		err = json.Unmarshal(bytes, &params)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResult { Code: http.StatusBadRequest, Message: "Invalid arguments." }
			json.NewEncoder(w).Encode(response)
			return
		}

		result, error := getTransaction(s.ElasticClient, params, s.getIndices())
		if error != nil {
			w.WriteHeader(error.Code)
			response := ErrorResult { Code: error.Code, Message: error.Error.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}
		//get missing fields from v1/chain/get_block
		txFromBlock, err := getTransactionFromBlock(result.BlockNum, result.Id)
		if err == nil {
			var receipt map[string]json.RawMessage
			err = json.Unmarshal(result.Trx["receipt"], &receipt)
			if err == nil {
				receipt["trx"] = txFromBlock
				bytes, err := json.Marshal(receipt)
				if err == nil {
					result.Trx["receipt"] = bytes
				}
			}
		}

		info, err := getInfo()
		if err == nil {
			result.LastIrreversibleBlock = info.LastIrreversibleBlockNum
		}

		b, err := json.Marshal(result)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}
		fmt.Fprintf(w, string(b))
	}
}

//handleGetKeyAccounts returns http handler that takes
//http.ResponseWriter and *http.Request as arguments
//it tries to parse parameters from request body
//and passes them to getKeyAccounts()
//The result of getKeyAccounts() is encoded and sent as a response
func (s *Server) handleGetKeyAccounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}

		var params GetKeyAccountsParams
		err = json.Unmarshal(bytes, &params)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResult { Code: http.StatusBadRequest, Message: "Invalid arguments." }
			json.NewEncoder(w).Encode(response)
			return
		}
		
		result, err := getKeyAccounts(s.ElasticClient, params, s.getIndices())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}
		b, err := json.Marshal(result)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}
		fmt.Fprintf(w, string(b))
	}
}

//handleGetControlledAccounts returns http handler that takes
//http.ResponseWriter and *http.Request as arguments
//it tries to parse parameters from request body
//and passes them to getControlledAccounts()
//The result of getControlledAccounts() is encoded and sent as a response
func (s *Server) handleGetControlledAccounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}

		var params GetControlledAccountsParams
		err = json.Unmarshal(bytes, &params)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := ErrorResult { Code: http.StatusBadRequest, Message: "Invalid arguments." }
			json.NewEncoder(w).Encode(response)
			return
		}

		result, err := getControlledAccounts(s.ElasticClient, params, s.getIndices())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}
		b, err := json.Marshal(result)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := ErrorResult { Code: http.StatusInternalServerError, Message: err.Error() }
			json.NewEncoder(w).Encode(response)
			return
		}
		fmt.Fprintf(w, string(b))
	}
}
package main

import (
	"errors"
	"encoding/json"
	"github.com/olivere/elastic"
	"context"
	"net/http"
	"bufio"
	"regexp"
	"strings"
	"math"
	"strconv"
	"sort"
)

const AccountsIndex          string = "accounts"
const BlocksIndex            string = "blocks"
const TransactionsIndex      string = "transactions"
const TransactionTracesIndex string = "transaction_traces"
const ActionTracesIndex      string = "action_traces"

const MaxQuerySize int = 10000


//get index list from ES and parse indices from it
//return a map where every prefix from input array is a key
//and a value is vector of corresponding indices
func getIndices(esUrl string, prefixes []string) map[string][]string {
	result := make(map[string][]string)
	resp, err := http.Get(esUrl + "/_cat/indices?v&s=index")
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	for _, prefix := range prefixes {
		r, err := regexp.Compile("\\s" + prefix + "-(\\d)*\\s")
		if err != nil {
			return result
		}
		for _, line := range lines {
			match := r.FindString(line)
			if len(match) != 0 {
				result[prefix] = append(result[prefix], strings.TrimSpace(match))
			}
		}
	}
	return result
}


func convertAbiToBytes(actionTraces []TransactionTraceActionTrace) {
	var actionTracesPtrs []*TransactionTraceActionTrace
	for i, _ := range actionTraces {
		actionTracesPtrs = append(actionTracesPtrs, &actionTraces[i])
	}
	trace := new(TransactionTraceActionTrace)
	for len(actionTracesPtrs) > 0 {
		trace = actionTracesPtrs[0]
		for i, _ := range trace.InlineTraces {
			actionTracesPtrs = append(actionTracesPtrs, &trace.InlineTraces[i])
		}
		actionTracesPtrs = actionTracesPtrs[1:len(actionTracesPtrs)]
		if trace.Act.Account == "eosio" && trace.Act.Name == "setabi" &&
			len(trace.Act.HexData) >= 20 { //in hex_data field encoded abi starts from 20 symbol
			data := trace.Act.HexData[20:]
			if m, ok := trace.Act.Data.(map[string]interface{}); ok {
				m["abi"] = data
			}
		}
	}
}


func findActionTrace(txTrace *TransactionTrace, actionSeq json.RawMessage) (*TransactionTraceActionTrace, error) {
	actionTraces := txTrace.ActionTraces
	trace := new(TransactionTraceActionTrace)
	for len(actionTraces) > 0 {
		trace = &actionTraces[0]
		actionTraces = append(actionTraces[1:len(actionTraces)], trace.InlineTraces...)
		var receipt map[string]*json.RawMessage
		err := json.Unmarshal(trace.Receipt, &receipt)
		if err != nil || receipt["global_sequence"] == nil {
			continue
		}
		var tmp interface{}
		var seq string
		var targetSeq string
		err = json.Unmarshal(*receipt["global_sequence"], &tmp)
		if err != nil {
			continue
		}
		if i, ok := tmp.(float64); ok {
			seq = strconv.FormatUint(uint64(i), 10)
		} else if s, ok := tmp.(string); ok {
			seq = s
		}
		err = json.Unmarshal(actionSeq, &tmp)
		if err != nil {
			continue
		}
		if i, ok := tmp.(float64); ok {
			targetSeq = strconv.FormatUint(uint64(i), 10)
		} else if s, ok := tmp.(string); ok {
			targetSeq = s
		}
		if seq == targetSeq {
			return trace, nil
		}
	}
	return nil, errors.New("Action trace not found in transaction trace")
}

func getActionTrace(client *elastic.Client, txId string, actionSeq json.RawMessage, indices map[string][]string) (json.RawMessage, error) {
	multiGet := client.MultiGet()
	for _, index := range indices[TransactionTracesIndexPrefix] {
		multiGet.Add(elastic.NewMultiGetItem().Index(index).Id(txId))
	}
	mgetResult, err := multiGet.Do(context.Background())
	if err != nil || mgetResult == nil || mgetResult.Docs == nil {
		return nil, err
	}
	var getResult *elastic.GetResult
	for _, doc := range mgetResult.Docs {
		if doc == nil || doc.Error != nil || !doc.Found {
			continue
		}
		getResult = doc
	}

	if getResult == nil || !getResult.Found || getResult.Source == nil {
		return nil, errors.New("Action trace not found")
	}
	var txTrace TransactionTrace
	err = json.Unmarshal(*getResult.Source, &txTrace)
	if err != nil {
		return nil, errors.New("Failed to parse ES response")
	}
	trace, err := findActionTrace(&txTrace, actionSeq)
	if err != nil {
		return nil, err
	}
	//replace json abi with bytes
	if trace.Act.Account == "eosio" && trace.Act.Name == "setabi" &&
		len(trace.Act.HexData) >= 20 {
		data := trace.Act.HexData[20:]
		if m, ok := trace.Act.Data.(map[string]interface{}); ok {
			m["abi"] = data
		}
	}
	convertAbiToBytes(trace.InlineTraces)
	bytes, err := json.Marshal(trace)
	if err != nil {
		return nil, errors.New("Failed to parse ES response")
	}
	return bytes, nil
}


func countActions(client *elastic.Client, params GetActionsParams, index string) (int64, error) {
	query := elastic.NewBoolQuery()
	query = query.Filter(elastic.NewMultiMatchQuery(params.AccountName, "receipt.receiver", "act.authorization.actor"))
	count, err := client.Count(index).
		Query(query).
		Do(context.Background())
	return count, err
}


func getActions(client *elastic.Client, params GetActionsParams, indices map[string][]string) (*GetActionsResult, error) {
	result := new(GetActionsResult)
	result.Actions = make([]Action, 0)
	ascOrder := true
	//deal with request params
	if *params.Pos == -1 {
		ascOrder = false
		if *params.Offset >= 0 {
			*params.Pos -= *params.Offset
			*params.Offset += 1
		} else {
			*params.Offset = int64(math.Abs(float64(*params.Offset - 1)))
		}
	} else {
		if *params.Offset >= 0 {
			*params.Offset += 1
		} else {
			*params.Pos += *params.Offset
			*params.Offset -= 1
			*params.Offset = int64(math.Abs(float64(*params.Offset)))
		}
	}
	if *params.Pos + *params.Offset <= 0 {
		return result, nil
	} else if *params.Pos < 0 {
		*params.Offset += *params.Pos
		*params.Pos = 0
	}

	//reverse index list if sort order is desc
	indexNum := len(indices[ActionTracesIndexPrefix])
	orderedIndices := make([]string, 0, indexNum)
	for i, _ := range indices[ActionTracesIndexPrefix] {
		if ascOrder {
			orderedIndices = append(orderedIndices, indices[ActionTracesIndexPrefix][i])
		} else {
			orderedIndices = append(orderedIndices, indices[ActionTracesIndexPrefix][indexNum-1-i])
		}
	}

	//find indices where actions from requested range are located
	var startPos *int
	var lastSize *int
	targetIndices := make([]string, 0)
	actionsPerTargetIndex := make([]int64, 0)
	actionsPerIndex := make([]int64, 0, indexNum)
	for _, index := range orderedIndices {
		count, _ := countActions(client, params, index)
		actionsPerIndex = append(actionsPerIndex, count)
	}
	totalActions := uint64(0)
	for _, value := range actionsPerIndex {
		totalActions += uint64(value)
	}
	counter := int64(0)
	i := 0
	for ; i < indexNum && counter + actionsPerIndex[i] < *params.Pos; i++ {
		counter += actionsPerIndex[i] //skip indices that contains action before Pos
	}
	if i < indexNum {
		startPos = new(int)
		*startPos = int(*params.Pos - counter)
	}
	for ; i < indexNum && counter + actionsPerIndex[i] < *params.Pos + *params.Offset; i++ {
		counter += actionsPerIndex[i]
		targetIndices = append(targetIndices, orderedIndices[i])
		actionsPerTargetIndex = append(actionsPerTargetIndex, actionsPerIndex[i])
	}
	if i < indexNum {
		targetIndices = append(targetIndices, orderedIndices[i])
		actionsPerTargetIndex = append(actionsPerTargetIndex, actionsPerIndex[i])
		lastSize = new(int)
		*lastSize = int(*params.Pos - counter + *params.Offset)
	}
	if len(targetIndices) == 0 {
		return result, nil
	}
	
	query := elastic.NewBoolQuery()
	query = query.Must(elastic.NewMultiMatchQuery(params.AccountName, "receipt.receiver", "act.authorization.actor"))
	msearch := client.MultiSearch()
	for i, index := range targetIndices {
		sreq := elastic.NewSearchRequest().
			Index(index).Query(query).
			Sort("receipt.global_sequence", ascOrder)
		if i == 0 && startPos != nil {
			sreq.From(*startPos)
		}
		if i == len(targetIndices) - 1 && lastSize != nil {
			sreq.Size(*lastSize)
		} else {
			sreq.Size(int(actionsPerTargetIndex[i]))
		}
		msearch.Add(sreq)
	}
	msearchResult, err := msearch.Do(context.Background())
	if err != nil || msearchResult == nil || msearchResult.Responses == nil {
		return nil, err
	}

	var searchHits []elastic.SearchHit
	for _, resp := range msearchResult.Responses {
		if resp == nil || resp.Error != nil {
			continue
		}
		for _, hit := range resp.Hits.Hits {
			if hit != nil && len(searchHits) < int(*params.Offset) {
				searchHits = append(searchHits, *hit)
			}
		}
		if len(searchHits) == int(*params.Offset) {
			break
		}
	}
	msearchResult.Responses = nil
	
	result.Actions = make([]Action, 0, len(searchHits))
	for i, hit := range searchHits {
		if hit.Source == nil {
			continue
		}

		var accountActionSeq uint64
		if ascOrder {
			accountActionSeq = uint64(*params.Pos) + uint64(i)
		} else {
			accountActionSeq = totalActions - (uint64(*params.Pos) + uint64(i + 1))
		}

		var actionTrace ActionTrace
		err = json.Unmarshal(*hit.Source, &actionTrace)
		if err != nil {
			continue
		}
		trace, err := getActionTrace(client, actionTrace.TrxId, actionTrace.Receipt.GlobalSequence, indices)
		if err != nil {
			continue
		}
		action := Action { GlobalActionSeq: actionTrace.Receipt.GlobalSequence,
			AccountActionSeq: accountActionSeq,
			BlockNum: actionTrace.BlockNum, BlockTime: actionTrace.BlockTime,
			ActionTrace: trace }
		result.Actions = append(result.Actions, action)
	}
	return result, nil
}


func getTransaction(client *elastic.Client, params GetTransactionParams, indices map[string][]string) (*GetTransactionResult, *ErrorWithCode) {
	mgetTx := client.MultiGet()
	mgetTxTrace := client.MultiGet()
	for _, index := range indices[TransactionsIndexPrefix] {
		mgetTx.Add(elastic.NewMultiGetItem().Index(index).Id(params.Id))
	}
	for _, index := range indices[TransactionTracesIndexPrefix] {
		mgetTxTrace.Add(elastic.NewMultiGetItem().Index(index).Id(params.Id))
	}
	mgetTxResult, err := mgetTx.Do(context.Background())
	if err != nil || mgetTxResult == nil || mgetTxResult.Docs == nil {
		error := new(ErrorWithCode)
		error.Error = err
		error.Code = 500
		return nil, error
	}
	mgetTxTraceResult, err := mgetTxTrace.Do(context.Background())
	if err != nil || mgetTxTraceResult == nil || mgetTxTraceResult.Docs == nil {
		error := new(ErrorWithCode)
		error.Error = err
		error.Code = 500
		return nil, error
	}

	var getTxResult *elastic.GetResult
	for _, doc := range mgetTxResult.Docs {
		if doc == nil || doc.Error != nil || !doc.Found {
			continue
		}
		getTxResult = doc
	}
	var getTxTraceResult *elastic.GetResult
	for _, doc := range mgetTxTraceResult.Docs {
		if doc == nil || doc.Error != nil || !doc.Found {
			continue
		}
		getTxTraceResult = doc
	}

	if getTxTraceResult == nil || !getTxTraceResult.Found {
		error := new(ErrorWithCode)
		error.Error = errors.New("Transaction not found.")
		error.Code = 404
		return nil, error
	}

	result, error := createTransaction(getTxResult, getTxTraceResult)
	if error != nil {
		return nil, error
	}
	result.Id = params.Id
	return result, nil
}


//gets info from transactions and transaction_traces indices
//and composes return value for get_transaction
func createTransaction(getTxResult *elastic.GetResult, getTxTraceResult *elastic.GetResult) (*GetTransactionResult, *ErrorWithCode) {
	//prepare data from transaction_traces index
	var txTrace TransactionTrace
	err := json.Unmarshal(*getTxTraceResult.Source, &txTrace)
	if err != nil {
		error := new(ErrorWithCode)
		error.Error = err
		error.Code = 500
		return nil, error
	}
	var status string
	err = json.Unmarshal(txTrace.Receipt["status"], &status)
	if err != nil {
		error := new(ErrorWithCode)
		error.Error = err
		error.Code = 500
		return nil, error
	}
	if status == "hard_fail" {
		error := new(ErrorWithCode)
		error.Error = errors.New("Transaction not found")
		error.Code = 404
		return nil, error
	}
	result := new(GetTransactionResult)
	result.Trx = make(map[string]json.RawMessage)
	result.BlockTime = txTrace.BlockTime
	result.BlockNum = txTrace.BlockNum
	//recursively replace json abi with bytes
	convertAbiToBytes(txTrace.ActionTraces)
	result.Traces, err = json.Marshal(txTrace.ActionTraces)
	if err != nil {
		error := new(ErrorWithCode)
		error.Error = err
		error.Code = 500
		return nil, error
	}
	result.Trx["receipt"], err = json.Marshal(txTrace.Receipt)
	if err != nil {
		error := new(ErrorWithCode)
		error.Error = err
		error.Code = 500
		return nil, error
	}
	
	//prepare data from transactions index
	if getTxResult != nil && getTxResult.Found {
		var transaction Transaction
		err = json.Unmarshal(*getTxResult.Source, &transaction)
		if err == nil {
			var actions []struct {
				Account                string `json:"account"`
				Name                   string `json:"name"`
				Authorization json.RawMessage `json:"authorization"`
				Data              interface{} `json:"data"`
				HexData                string `json:"hex_data,omitempty"`
			}
			err = json.Unmarshal(transaction.Actions, &actions)
			if err == nil {
				for i, _ := range actions {
					//actions contain abi in json format so we need to extract abi from hex_data field
					if actions[i].Account == "eosio" && actions[i].Name == "setabi" &&
						len(actions[i].HexData) >= 20 { //in hex_data field encoded abi starts from 20 symbol
						data := actions[i].HexData[20:]
						if m, ok := actions[i].Data.(map[string]interface{}); ok {
							m["abi"] = data
						}
					}
				}
				bytes, err := json.Marshal(actions)
				if err == nil {
					transaction.Actions = bytes
				}
			}
			trx := make(map[string]json.RawMessage)
			trx["expiration"] = transaction.Expiration
			trx["ref_block_num"] = transaction.RefBlockNum
			trx["ref_block_prefix"] = transaction.RefBlockPrefix
			trx["max_net_usage_words"] = transaction.MaxNetUsageWords
			trx["max_cpu_usage_ms"] = transaction.MaxCpuUsageMs
			trx["delay_sec"] = transaction.DelaySec
			trx["context_free_actions"] = transaction.ContextFreeActions
			trx["actions"] = transaction.Actions
			trx["transaction_extensions"] = transaction.TransactionExtensions
			trx["signatures"] = transaction.Signatures
			trx["context_free_data"] = transaction.ContextFreeData
			byteTrx, err := json.Marshal(trx)
			if err != nil {
				error := new(ErrorWithCode)
				error.Error = err
				error.Code = 500
				return nil, error
			}
			result.Trx["trx"] = byteTrx
		}
	}
	return result, nil
}


func getKeyAccounts(client *elastic.Client, params GetKeyAccountsParams, indices map[string][]string) (*GetKeyAccountsResult, error) {
	query := elastic.NewBoolQuery()
	query = query.Filter(elastic.NewMatchQuery("pub_keys.key", params.PublicKey))
	msearch := client.MultiSearch()
	for _, index := range indices[AccountsIndexPrefix] {
		msearch.Add(elastic.NewSearchRequest().Index(index).Query(query).Size(MaxQuerySize))
	}
	msearchResult, err := msearch.Do(context.Background())
	if err != nil || msearchResult == nil || msearchResult.Responses == nil {
		return nil, err
	}
	var searchHits []elastic.SearchHit
	for _, resp := range msearchResult.Responses {
		if resp == nil || resp.Error != nil {
			continue
		}
		for _, hit := range resp.Hits.Hits {
			if hit != nil {
				searchHits = append(searchHits, *hit)
			}
		}
	}

	result := new(GetKeyAccountsResult)
	result.AccountNames = make([]string, 0, len(searchHits))
	for _, hit := range searchHits {
		if hit.Source == nil {
			continue
		}
		var account Account
		err := json.Unmarshal(*hit.Source, &account)
		if err != nil {
			return nil, errors.New("Failed to parse ES response")
		}
		result.AccountNames = append(result.AccountNames, account.Name)
	}
	sort.Strings(result.AccountNames)
	return result, nil
}


func getControlledAccounts(client *elastic.Client, params GetControlledAccountsParams, indices map[string][]string) (*GetControlledAccountsResult, error) {
	query := elastic.NewBoolQuery()
	query = query.Filter(elastic.NewMatchQuery("account_controls.name.keyword", params.ControllingAccount))
	msearch := client.MultiSearch()
	for _, index := range indices[AccountsIndexPrefix] {
		msearch.Add(elastic.NewSearchRequest().Index(index).Query(query).Size(MaxQuerySize))
	}
	msearchResult, err := msearch.Do(context.Background())
	if err != nil || msearchResult == nil || msearchResult.Responses == nil {
		return nil, err
	}
	var searchHits []elastic.SearchHit
	for _, resp := range msearchResult.Responses {
		if resp == nil || resp.Error != nil {
			continue
		}
		for _, hit := range resp.Hits.Hits {
			if hit != nil {
				searchHits = append(searchHits, *hit)
			}
		}
	}

	result := new(GetControlledAccountsResult)
	result.ControlledAccounts = make([]string, 0, len(searchHits))
	for _, hit := range searchHits {
		if hit.Source == nil {
			continue
		}
		var account Account
		err := json.Unmarshal(*hit.Source, &account)
		if err != nil {
			return nil, errors.New("Failed to parse ES response")
		}
		result.ControlledAccounts = append(result.ControlledAccounts, account.Name)
	}
	sort.Strings(result.ControlledAccounts)
	return result, nil
}
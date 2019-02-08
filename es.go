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

const MaxQuerySize          int = 10000
const MaxFindActionsResults int = 100


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


//takes pointer to TransactionTraceActionTrace
//returns slice of pointers to TransactionTraceActionTrace
//that located inside of trace including given trace
func expandTraces(trace *TransactionTraceActionTrace) ([]*TransactionTraceActionTrace) {
	var traces []*TransactionTraceActionTrace
	if trace == nil {
		return traces
	}
    tmp := []*TransactionTraceActionTrace {trace}
    for len(tmp) > 0 {
        tPtr := tmp[0]
        tmp = tmp[1:]

        traces = append(traces, tPtr)

        var inlineTraces []*TransactionTraceActionTrace
        for i, _ := range tPtr.InlineTraces {
            inlineTraces = append(inlineTraces, &tPtr.InlineTraces[i])
        }
        tmp = append(inlineTraces, tmp...)
	}
	return traces
}


//takes pointer to TransactionTraceActionTrace
//if given action is eosio:setabi
//replaces data->abi as json object with same abi as bytes
//abi as bytes is substring of hex_data (from 20 symbol to the end of string)
func convertAbiToBytes(trace *TransactionTraceActionTrace) {
	if trace == nil {
		return
	}
	if trace.Act.Account == "eosio" && trace.Act.Name == "setabi" &&
		len(trace.Act.HexData) >= 20 { //in hex_data field encoded abi starts from 20 symbol
		data := trace.Act.HexData[20:]
		if m, ok := trace.Act.Data.(map[string]interface{}); ok {
			m["abi"] = data
		}
	}
}


func findActionTrace(txTrace *TransactionTrace, actionSeq json.RawMessage) (*TransactionTraceActionTrace) {
	if txTrace == nil {
		return nil
	}
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
			return trace
		}
	}
	return nil
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
	accountFilter := elastic.NewMultiMatchQuery(params.AccountName, "receipt.receiver", "act.authorization.actor")
	exceptFilter := elastic.NewBoolQuery().MustNot(elastic.NewExistsQuery("except"))
	query = query.Filter(accountFilter, exceptFilter)
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

	//get info from action_traces*
	actionTraces := make([]ActionTrace, 0)
	txIds := make([]string, 0)
	mapTxTraces := make(map[string]*TransactionTrace)
	for _, resp := range msearchResult.Responses {
		if resp == nil || resp.Error != nil {
			continue
		}
		for _, hit := range resp.Hits.Hits {
			if hit == nil || hit.Source == nil {
				continue
			}
			var actionTrace ActionTrace
			if err = json.Unmarshal(*hit.Source, &actionTrace); err == nil {
				actionTraces = append(actionTraces, actionTrace)
				_, txEncountered := mapTxTraces[actionTrace.TrxId]
				if !txEncountered {
					mapTxTraces[actionTrace.TrxId] = new(TransactionTrace)
					txIds = append(txIds, actionTrace.TrxId)
				}
			}
		}
	}
	msearchResult.Responses = nil
	//get all transaction_traces that contain collected action traces
	txTraces, err := getTransactionTraces(client, txIds, indices[TransactionTracesIndexPrefix])
	if err != nil {
		return nil, err
	}
	for _, txTrace := range txTraces {
		mapTxTraces[txTrace.Id] = txTrace
	}
	for i, at := range actionTraces {
		var accountActionSeq uint64
		if ascOrder {
			accountActionSeq = uint64(*params.Pos) + uint64(i)
		} else {
			accountActionSeq = totalActions - (uint64(*params.Pos) + uint64(i + 1))
		}

		trace := findActionTrace(mapTxTraces[at.TrxId], at.Receipt.GlobalSequence)
		//replace json abi with bytes
		expandedTraces := expandTraces(trace)
		for _, tPtr := range expandedTraces {
			convertAbiToBytes(tPtr)
		}
		bytes, _ := json.Marshal(trace)
		action := Action { GlobalActionSeq: at.Receipt.GlobalSequence,
			AccountActionSeq: &accountActionSeq,
			BlockNum: at.BlockNum, BlockTime: at.BlockTime,
			ActionTrace: bytes }
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
	var traces []*TransactionTraceActionTrace
	for i, _ := range txTrace.ActionTraces {
		traces = append(traces, expandTraces(&txTrace.ActionTraces[i])...)
	}
	needExpandTraces := len(txTrace.ActionTraces) != len(traces)
	if needExpandTraces {
		txTrace.ActionTraces = make([]TransactionTraceActionTrace, 0, len(traces))
	}
	for _, tPtr := range traces {
		convertAbiToBytes(tPtr)
		if needExpandTraces {
			txTrace.ActionTraces = append(txTrace.ActionTraces, *tPtr)
		}
	}
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

//performs query on ES
//query contains filter on act.data field
//if account_name was provided then the filter on receipt.receiver and act.authorization.actor is also included
//if at least one of from_date or to_date was provided then the filter on block_time is included
//if last_days was provided then the filter on block_time is included
func findActionsByData(client *elastic.Client, params FindActionsParams, indices map[string][]string) (*FindActionsResult, error) {
	query := elastic.NewBoolQuery()
	filters := make([]elastic.Query, 0)
	filters = append(filters, elastic.NewMatchQuery("act.data", params.Data))
	if params.AccountName != "" {
		filters = append(filters, elastic.NewMultiMatchQuery(params.AccountName, "receipt.receiver", "act.authorization.actor"))
	}
	if params.FromDate != "" || params.ToDate != "" {
		rangeQ := elastic.NewRangeQuery("block_time")
		if params.FromDate != "" {
			rangeQ.Gte(params.FromDate)
		}
		if params.ToDate != "" {
			rangeQ.Lte(params.ToDate)
		}
		filters = append(filters, rangeQ)
	}
	if params.LastDays != nil {
		rangeQ := elastic.NewRangeQuery("block_time").
			Gte("now-" + strconv.FormatUint(uint64(*params.LastDays), 10) + "d/d")
		filters = append(filters, rangeQ)
	}
	filters = append(filters, elastic.NewBoolQuery().MustNot(elastic.NewExistsQuery("except")))
	query = query.Filter(filters...)
	searchResult, err := client.Search().
		Index(ActionTracesIndexPrefix + "*").
		Query(query).
		Sort("receipt.global_sequence", true).
		Size(MaxFindActionsResults).
		Do(context.Background())
	if err != nil || searchResult == nil || searchResult.Hits == nil {
		return nil, err
	}

	result := new(FindActionsResult)
	result.Actions = make([]Action, 0, len(searchResult.Hits.Hits))
	if len(searchResult.Hits.Hits) == 0 {
		return result, nil
	}
	//get info from action_traces*
	actionTraces := make([]ActionTrace, 0, len(searchResult.Hits.Hits))
	txIds := make([]string, 0)
	mapTxTraces := make(map[string]*TransactionTrace)
	for _, hit := range searchResult.Hits.Hits {
		if hit == nil || hit.Source == nil {
			continue
		}
		var actionTrace ActionTrace
		if err = json.Unmarshal(*hit.Source, &actionTrace); err == nil {
			actionTraces = append(actionTraces, actionTrace)
			_, txEncountered := mapTxTraces[actionTrace.TrxId]
			if !txEncountered {
				mapTxTraces[actionTrace.TrxId] = new(TransactionTrace)
				txIds = append(txIds, actionTrace.TrxId)
			}
		}
	}
	//get all transaction_traces that contain collected action traces
	txTraces, err := getTransactionTraces(client, txIds, indices[TransactionTracesIndexPrefix])
	if err != nil {
		return nil, err
	}
	for _, txTrace := range txTraces {
		mapTxTraces[txTrace.Id] = txTrace
	}
	for _, at := range actionTraces {
		trace := findActionTrace(mapTxTraces[at.TrxId], at.Receipt.GlobalSequence)
		//replace json abi with bytes
		expandedTraces := expandTraces(trace)
		for _, tPtr := range expandedTraces {
			convertAbiToBytes(tPtr)
		}
		bytes, _ := json.Marshal(trace)
		action := Action { GlobalActionSeq: at.Receipt.GlobalSequence,
			BlockNum: at.BlockNum, BlockTime: at.BlockTime,
			ActionTrace: bytes }
		result.Actions = append(result.Actions, action)
	}
	return result, nil
}

//takes elasticsearch client, list of transactions to get and
//slice of transaction_traces* indices from which tx traces will be retrieved
//performs multi get request on ES
//unmarshals results to TransactionTrace structs and returns them
func getTransactionTraces(client *elastic.Client, txIds []string, txTracesIndices []string) ([]*TransactionTrace, error) {
	result := make([]*TransactionTrace, 0)
	if len(txIds) == 0 {
		return result, nil
	}
	multiGet := client.MultiGet()
	for _, index := range txTracesIndices {
		for _, txId := range txIds {
			multiGet.Add(elastic.NewMultiGetItem().Index(index).Id(txId))
		}
	}
	mgetResult, err := multiGet.Do(context.Background())
	if err != nil || mgetResult == nil || mgetResult.Docs == nil {
		return nil, err
	}
	for _, doc := range mgetResult.Docs {
		if doc == nil || doc.Error != nil || !doc.Found || doc.Source == nil {
			continue
		}
		
		var txTrace TransactionTrace
		err = json.Unmarshal(*doc.Source, &txTrace)
		if err != nil {
			continue
		}
		result = append(result, &txTrace)
	}
	return result, nil
}
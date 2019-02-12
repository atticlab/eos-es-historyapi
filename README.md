# EOS ESHistoryAPI - GO (beta)
EOS History API for Elasticsearch cluster on GO.
  
During the benchmark testing, real query traffic was redirected to this application.  
The application works consistently at > 1000 simultaneous connections.  

## Installation
#### Get source code
```sh
$ cd $GOPATH/src
$ git clone https://github.com/atticlab/eos-es-historyapi.git
$ cd eos-es-historyapi/
```
#### 
#### Create config.json
In the project directory create file config.json.  
"port" property is for the port on which the server will listen.  
"elastic_url" property is for the url of elasticsearch cluster.  
"seed_node" property is for the url of the node with chain_api_plugin enabled.  
For example:

    {
        "port": 9000,
        "elastic_url": "http://127.0.0.1:9201",
        "seed_node": "https://proxy.eosnode.tools"
    }  
  
The "seed_node" parameter is needed by the application to connect to the node and receive transactions.trx that are not in the Elasticsearch data.  

#### Create .env file
In project directory create file .env  
Change path to GO directory to your path  
For example:
```
GOPATH=/home/eosuser/go  
NAMEREPO=eos-es-historyapi  
MIDDLEWARE_SOURCE_PORT=9000  
MIDDLEWARE_DEST_PORT=9000  
```
####
## Application
#### Build application
```sh
$make build-app
```
#### Run application
```sh
$make start
```
### Stop application
```sh
$make stop
```
## Docker
#### Build docker
```sh
$make build-docker
```
#### create docker-compose
```sh
$make create-compose
```
#### Run docker-compose
```sh
$make docker-start
```
#### Stop docker-compose
```sh
$make docker-stop
```
#### 
## Usage
This API supports following GET and POST requests:  

#### /v1/history/get_actions
Requires json body with the following properties:  
account_name - name of the eos account. This field is required.  
pos - position in a list of account actions sorted by global_sequence (e.g. in chronological order). This field is not required.  
offset - number of actions to return. This field is not required.  
Example of request body:

    {
        "account_name": "eosio",
        "pos": 0,
        "offset": 10
    }
  
Returns json with the following properties:  
actions - array of actions of a given account  
#### /v1/history/get_transaction
Requires json body with the following properties:  
id - id of a transaction.  
Example of request body:

    {
        "id": "e6c814f9ba58e2aedd654abfdefc99c98f3e4bf5f20e4820b7d212f38f1f6f13"
    }
  
Returns json with the following properties:  
id - id of a transaction.  
trx - transaction.  
block_time - timestamp of the block which contains the requested transaction.  
block_num - number of the block which contains the requested transaction.  
traces - traces of the transaction.  
#### /v1/history/get_key_accounts
Requires json body with the following properties:  
public_key - public key of account
Example of request body:

    {
        "public_key": "EOS81Z5dYnSnfzdNFViMcGQoYUqrgZSdKJ69mvsnp2CLH2ufqX8Y9"
    }
  
Returns json with the following properties:  
account_names - array of accounts that have a requested key  
#### /v1/history/get_controlled_accounts
Requires json body with the following properties:  
controlling_account - name of the eos account  
Example of request body:

    {
        "controlling_account": "eosio"
    }
  
Returns json with the following properties:  
controlled_accounts - array of accounts controlled by a requested account  

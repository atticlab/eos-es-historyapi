# Test script
Python script to compare ES history API with fullnode API

## Usage
#### Create config.json
In test directory create file config.json.  
"input_file" property is for the config filename.  
"fullnode_api_url" property is for the url of fullnode api.  
"es_api_url" property is for the url of elasticsearch api.  
For example:

    {
        "input_file": "tests.json",
        "fullnode_api_url": "https://proxy.eosnode.tools/",
        "es_api_url": "http://eosbp-0.atticlab.net/"
    }
#### Run
```sh
$ python test.py
```
Results will be saved to out.json
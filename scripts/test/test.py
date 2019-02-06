import os
import requests
import json
from jsondiff import diff


outputFilename = 'out.json'


class HistoryClient:
    def __init__(self, url):
        if url[-1] != '/':
            url = url + '/'
        self.url = url

    def performRequest(self, method, edge, params):
        if edge[0] == '/':
            edge = edge[1]
        request = self.url + edge
        response = requests.request(method, request, json=params)
        return response.json()


def runTest(fullnodeUrl, esUrl, data, addSources = False):
    client = HistoryClient(fullnodeUrl)
    clientES = HistoryClient(esUrl)

    method = data["method"]
    edge = data["edge"]
    result = {
        'edge': edge,
        'results': []
    }
    print('Testing ' + edge)
    count = 0
    success = 0
    for param in data["params"]:
        count += 1
        json1 = client.performRequest(method, edge, param)
        json2 = clientES.performRequest(method, edge, param)
        d = diff(json1, json2, dump=True)
        d = json.loads(d)
        res = {
            'test number': count,
            'params': param
        }
        if d:
            res['passed'] = False
            res['diff'] = d
            if addSources:
                res['source1'] = json1
                res['source2'] = json2
        else:
            res['passed'] = True
            success += 1
        result['results'].append(res)
    return result


def main():
    config = {}
    tests = []
    with open('config.json', 'r') as fin:
        config = json.load(fin)
    with open(config['input_file'], 'r') as fin:
        tests = json.load(fin)
    if os.path.exists(outputFilename):
        os.remove(outputFilename)

    results = []
    for test in tests:
        if 'enabled' in test and not test['enabled']:
            continue
        
        try:
            result = runTest(config['fullnode_api_url'], config['es_api_url'], test, False)
            results.append(result)
        except Exception:
            print('Exception caught')
    
    with open(outputFilename, 'a') as fout:
        fout.write(json.dumps(results, indent=4))


if __name__ == '__main__':
    main()
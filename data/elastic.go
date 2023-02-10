package data

type ElasticResult struct {
	Hits struct {
		Hits []*ElasticEntry `json:"hits"`
	} `json:"hits"`
}

type ElasticEntry struct {
	Hash   string `json:"_id"`
	Source struct {
		Nonce    uint64 `json:"nonce"`
		Value    string `json:"value"`
		Receiver string `json:"receiver"`
		Sender   string `json:"sender"`
		Data     []byte `json:"data"`
		Status   string `json:"status"`
	} `json:"_source"`
}

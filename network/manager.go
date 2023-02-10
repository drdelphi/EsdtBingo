package network

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/DrDelphi/EsdtBingoBot/data"
	"github.com/DrDelphi/EsdtBingoBot/utils"
	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/elrond-sdk-erdgo/blockchain"
	"github.com/ElrondNetwork/elrond-sdk-erdgo/builders"
	sdkData "github.com/ElrondNetwork/elrond-sdk-erdgo/data"
	"github.com/ElrondNetwork/elrond-sdk-erdgo/interactors"
)

var log = logger.GetOrCreate("network")

// NetworkManager - holds the required fields of a network manager
type NetworkManager struct {
	NetworkConfig *sdkData.NetworkConfig
	cfg           *data.AppConfig

	fDenomination *big.Float
	proxy         blockchain.Proxy
	conv          core.PubkeyConverter
}

// NewNetworkManager - creates a new NetworkManager object
func NewNetworkManager(cfg *data.AppConfig) (*NetworkManager, error) {
	proxy := blockchain.NewElrondProxy(cfg.Network.Proxy, nil)

	networkConfig, err := proxy.GetNetworkConfig(context.Background())
	if err != nil {
		log.Error("can not get network config from proxy", "error", err)
		return nil, err
	}

	fDenomination := big.NewFloat(1)
	for i := 0; i < networkConfig.Denomination; i++ {
		fDenomination.Mul(fDenomination, big.NewFloat(10))
	}

	conv, err := pubkeyConverter.NewBech32PubkeyConverter(32, log)
	if err != nil {
		log.Error("can not create converter", "error", err)
		return nil, err
	}

	utils.ExtractNumbersPK = utils.GetPrivateKeyFromSeed(0)
	if err != nil {
		log.Error("can not read pem file", "error", err)
		return nil, err
	}

	networkManager := &NetworkManager{
		NetworkConfig: networkConfig,
		cfg:           cfg,
		fDenomination: fDenomination,
		proxy:         proxy,
		conv:          conv,
	}

	return networkManager, nil
}

func (nm *NetworkManager) getScOneResult(function string) ([]byte, error) {
	req := &sdkData.VmValueRequest{
		Address:  nm.cfg.ContractAddress,
		FuncName: function,
	}
	res, err := nm.proxy.ExecuteVMQuery(context.Background(), req)
	if err != nil {
		log.Error("getScOneResult", "function", function, "error", err)
		return nil, err
	}

	if len(res.Data.ReturnData) == 0 {
		return nil, errEmptyResponse
	}

	return res.Data.ReturnData[0], nil
}

func (nm *NetworkManager) getScMultiResults(function string, args []string) ([][]byte, error) {
	req := &sdkData.VmValueRequest{
		Address:  nm.cfg.ContractAddress,
		FuncName: function,
		Args:     args,
	}
	res, err := nm.proxy.ExecuteVMQuery(context.Background(), req)
	if err != nil {
		log.Error("getScMultiResults", "function", function, "args", args, "error", err)
		return nil, err
	}

	if len(res.Data.ReturnData) == 0 {
		return nil, errEmptyResponse
	}

	return res.Data.ReturnData, nil
}

func (nm *NetworkManager) getUint64(function string) (uint64, error) {
	bytes, err := nm.getScOneResult(function)
	if err != nil {
		return 0, err
	}

	if len(bytes) == 0 {
		return 0, nil
	}

	return big.NewInt(0).SetBytes(bytes).Uint64(), nil
}

func (nm *NetworkManager) getBigInt(function string) (*big.Int, error) {
	bytes, err := nm.getScOneResult(function)
	if err != nil {
		return nil, err
	}

	if len(bytes) == 0 {
		return big.NewInt(0), nil
	}

	return big.NewInt(0).SetBytes(bytes), nil
}

func (nm *NetworkManager) GetRound() (uint64, error) {
	return nm.getUint64("getRound")
}

func (nm *NetworkManager) GetRoundTickets() (uint64, error) {
	return nm.getUint64("getRoundTickets")
}

func (nm *NetworkManager) GetNumbersToExtract() (uint64, error) {
	return nm.getUint64("getNumbersToExtract")
}

func (nm *NetworkManager) GetLastExtractedNumbers() ([]byte, error) {
	res, err := nm.getBigInt("getLastExtractedNumbers")
	if err != nil {
		return nil, err
	}

	return utils.BytesToNumbers(res.Bytes()), err
}

func (nm *NetworkManager) GetTicketPrice() (float64, error) {
	bigInt, err := nm.getBigInt("getTicketPrice")
	if err != nil {
		return 0, err
	}

	fPrice, _ := big.NewFloat(0).SetString(bigInt.String())
	fPrice.Quo(fPrice, nm.fDenomination)
	price, _ := fPrice.Float64()

	return price, nil
}

func (nm *NetworkManager) GetPrizesMultipliers() (uint64, uint64, uint64, error) {
	bingo, err := nm.getUint64("getBingoPrizeMultiplier")
	if err != nil {
		return 0, 0, 0, err
	}

	twoLines, err := nm.getUint64("getTwoLinesPrizeMultiplier")
	if err != nil {
		return 0, 0, 0, err
	}

	oneLine, err := nm.getUint64("getOneLinePrizeMultiplier")
	if err != nil {
		return 0, 0, 0, err
	}

	return bingo, twoLines, oneLine, nil
}

func (nm *NetworkManager) GetStatistics() (uint64, uint64, uint64, uint64, error) {
	tickets, err := nm.getUint64("getAllTimeTickets")
	if err != nil {
		return 0, 0, 0, 0, err
	}

	bingo, err := nm.getUint64("getAllTimeBingo")
	if err != nil {
		return 0, 0, 0, 0, err
	}

	twoLines, err := nm.getUint64("getAllTimeTwoLines")
	if err != nil {
		return 0, 0, 0, 0, err
	}

	oneLine, err := nm.getUint64("getAllTimeOneLine")
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return tickets, bingo, twoLines, oneLine, nil
}

func (nm *NetworkManager) GetDeadline() (int64, error) {
	res, err := nm.getUint64("getDeadline")

	return int64(res)*nm.NetworkConfig.RoundDuration/1000 + nm.NetworkConfig.StartTime, err
}

func (nm *NetworkManager) GetRoundDuration() (int64, error) {
	res, err := nm.getUint64("getRoundDuration")

	return int64(res) * nm.NetworkConfig.RoundDuration / 1000, err
}

func (nm *NetworkManager) GetStatus() (int, error) {
	res, err := nm.getUint64("getStatus")

	return int(res), err
}

func (nm *NetworkManager) GetPlayers() ([]string, error) {
	res, err := nm.getScMultiResults("getPlayers", nil)
	if err != nil {
		return nil, err
	}

	players := make([]string, 0)
	for _, pubkey := range res {
		address := nm.conv.Encode(pubkey)
		players = append(players, address)
	}

	return players, nil
}

func (nm *NetworkManager) GetPlayerTickets(player string) ([]*data.Ticket, error) {
	pubkey, err := nm.conv.Decode(player)
	if err != nil {
		return nil, err
	}

	res, err := nm.getScMultiResults("getUserTickets", []string{hex.EncodeToString(pubkey)})
	if err != nil {
		return nil, err
	}

	tickets := make([]*data.Ticket, 0)
	for _, ticket := range res {
		numbers := utils.BytesToTicket(ticket[4:])
		tickets = append(tickets, numbers)
	}

	return tickets, nil
}

func (nm *NetworkManager) GetContractInfo() (*data.ContractInfo, error) {
	var err error
	info := data.ContractInfo{}

	if info.GameRound, err = nm.GetRound(); err != nil {
		return nil, err
	}

	if info.RoundTickets, err = nm.GetRoundTickets(); err != nil {
		return nil, err
	}

	if info.NumbersToExtract, err = nm.GetNumbersToExtract(); err != nil {
		return nil, err
	}

	if info.LastExtractedNumbers, err = nm.GetLastExtractedNumbers(); err != nil {
		return nil, err
	}

	if info.TicketPrice, err = nm.GetTicketPrice(); err != nil {
		return nil, err
	}

	if info.PrizesMultipliers.Bingo, info.PrizesMultipliers.TwoLines, info.PrizesMultipliers.OneLine, err = nm.GetPrizesMultipliers(); err != nil {
		return nil, err
	}

	if info.Deadline, err = nm.GetDeadline(); err != nil {
		return nil, err
	}

	if info.RoundDuration, err = nm.GetRoundDuration(); err != nil {
		return nil, err
	}

	if info.Status, err = nm.GetStatus(); err != nil {
		return nil, err
	}

	if info.Statistics.TotalTickets, info.Statistics.TotalBingo, info.Statistics.TotalTwoLines, info.Statistics.TotalOneLine, err = nm.GetStatistics(); err != nil {
		return nil, err
	}

	if info.TickerIdentifier, err = nm.GetTickerIdentifier(); err != nil {
		return nil, err
	}

	return &info, nil
}

func (nm *NetworkManager) GetBalance(address string) (float64, error) {
	pubkey, err := nm.conv.Decode(address)
	if err != nil {
		log.Error("getBalance - Decode", "address", address, "error", err)
		return 0, err
	}

	account, err := nm.proxy.GetAccount(context.Background(), sdkData.NewAddressFromBytes(pubkey))
	if err != nil {
		log.Error("getBalance - GetAccount", "address", address, "error", err)
		return 0, err
	}

	balance, err := account.GetBalance(nm.NetworkConfig.Denomination)
	if err != nil {
		log.Error("getBalance - GetBalance", "address", address, "error", err)
		return 0, err
	}

	return balance, nil
}

func (nm *NetworkManager) GetTokenBalance(address, tokenIdentifier string) (float64, error) {
	endpoint := fmt.Sprintf("%s/address/%s/esdt/%s", nm.cfg.Network.Proxy, address, tokenIdentifier)
	bytes, err := utils.GetHTTP(endpoint)
	if err != nil {
		log.Error("getTokenBalance", "error", err)
		return 0, err
	}

	res := data.EsdtBalanceResponse{}
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		log.Error("getTokenBalance", "error", err)
		return 0, err
	}

	fBalance, ok := big.NewFloat(0).SetString(res.Data.TokenData.Balance)
	if !ok {
		return 0, errInvalidResponse
	}

	prop, err := nm.GetTokenProperties(tokenIdentifier)
	if err != nil {
		log.Error("getTokenBalance - getTokenProperties", "error", err)
		return 0, err
	}

	fDenom := big.NewFloat(1)
	for i := 0; i < int(prop.Decimals); i++ {
		fDenom.Mul(fDenom, big.NewFloat(10))
	}

	fBalance.Quo(fBalance, fDenom)
	ret, _ := fBalance.Float64()

	return ret, nil
}

func (nm *NetworkManager) GetAddressNonce(address string) (uint64, error) {
	pubkey, err := nm.conv.Decode(address)
	if err != nil {
		log.Error("getBalance - Decode", "address", address, "error", err)
		return 0, err
	}

	account, err := nm.proxy.GetAccount(context.Background(), sdkData.NewAddressFromBytes(pubkey))
	if err != nil {
		log.Error("getBalance - GetAccount", "address", address, "error", err)
		return 0, err
	}

	return account.Nonce, nil
}

func (nm *NetworkManager) SendTransaction(privateKey []byte, amount float64, gasLimit uint64, function string, nonce uint64) (string, error) {
	ep := blockchain.NewElrondProxy(nm.cfg.Network.Proxy, nil)
	w := interactors.NewWallet()
	builder, _ := builders.NewTxBuilder(blockchain.NewTxSigner())
	ti, err := interactors.NewTransactionInteractor(ep, builder)
	if err != nil {
		log.Error("error creating transaction interactor", "error", err)
		return "", err
	}

	senderAddress, err := w.GetAddressFromPrivateKey(privateKey)
	if err != nil {
		log.Error("unable to load the address from the private key", "error", err)
		return "", err
	}

	txArgs, err := ep.GetDefaultTransactionArguments(context.Background(), senderAddress, nm.NetworkConfig)
	if err != nil {
		log.Error("unable to prepare the transaction creation arguments", "error", err)
		return "", err
	}

	if nonce < utils.AutoNonce {
		txArgs.Nonce = nonce
	}

	txArgs.GasLimit = gasLimit
	txArgs.RcvAddr = nm.cfg.ContractAddress
	txArgs.Data = []byte(function)

	bValue := big.NewFloat(amount)
	bValue.Mul(bValue, nm.fDenomination)
	iValue, _ := bValue.Int(nil)
	txArgs.Value = iValue.String()

	tx, err := ti.ApplySignatureAndGenerateTx(privateKey, txArgs)
	if err != nil {
		log.Error("unable to sign transaction", "error", err)
		return "", err
	}

	return ti.SendTransaction(context.Background(), tx)
}

func (nm *NetworkManager) GetTransactionInfo(hash string) (*data.ElasticEntry, error) {
	endpoint := fmt.Sprintf("%s/transactions/_search?size=1&q=_id:%s", nm.cfg.Network.Indexer, hash)
	bytes, err := utils.GetHTTP(endpoint)
	if err != nil {
		return nil, err
	}

	res := &data.ElasticResult{}
	err = json.Unmarshal(bytes, res)
	if err != nil {
		return nil, err
	}

	if len(res.Hits.Hits) != 1 {
		return nil, errInvalidResponse
	}

	return res.Hits.Hits[0], nil
}

func (nm *NetworkManager) GetTransactionScrs(hash string) ([]*data.ElasticEntry, error) {
	endpoint := fmt.Sprintf("%s/scresults/_search?size=1000&q=originalTxHash:%s", nm.cfg.Network.Indexer, hash)
	bytes, err := utils.GetHTTP(endpoint)
	if err != nil {
		return nil, err
	}

	res := &data.ElasticResult{}
	err = json.Unmarshal(bytes, res)
	if err != nil {
		return nil, err
	}

	if len(res.Hits.Hits) == 0 {
		return nil, errInvalidResponse
	}

	return res.Hits.Hits, nil
}

func (nm *NetworkManager) GetTickerIdentifier() (string, error) {
	res, err := nm.getScOneResult("getTokenIdentifier")
	if err != nil {
		return "", err
	}

	return string(res), nil
}

func (nm *NetworkManager) GetTokenProperties(ticker string) (*data.ESDT, error) {
	req := &sdkData.VmValueRequest{
		Address:  "erd1qqqqqqqqqqqqqqqpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzllls8a5w6u",
		FuncName: "getTokenProperties",
		Args:     []string{hex.EncodeToString([]byte(ticker))},
	}
	res, err := nm.proxy.ExecuteVMQuery(context.Background(), req)
	if err != nil {
		log.Warn("unable to get token properties", "token", ticker, "error", err)
		return nil, err
	}

	if len(res.Data.ReturnData) < 6 {
		log.Warn("invalid token properties", "len", len(res.Data.ReturnData))
		return nil, errors.New("invalid get token properties response")
	}

	token := &data.ESDT{}
	token.Ticker = ticker
	token.ShortTicker = strings.Split(ticker, "-")[0]
	token.Name = string(res.Data.ReturnData[0])
	decimals := string(res.Data.ReturnData[5])
	if !strings.HasPrefix(decimals, "NumDecimals-") {
		log.Warn("invalid token decimals", "token", ticker, "decimals", decimals, "error", err)
		return nil, errors.New("invalid token decimals")
	}

	decimalsStr := strings.TrimPrefix(decimals, "NumDecimals-")
	token.Decimals, _ = strconv.ParseUint(decimalsStr, 10, 64)

	return token, nil
}

func (nm *NetworkManager) GetEgldDenomination() *big.Float {
	return nm.fDenomination
}

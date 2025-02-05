package backtest

import (
	"backend/socket"
	"backend/utils"
	"time"
)

type TriggeredEvent struct {
	AlgoName   string
	SecurityID int
	Timestamp  time.Time
}

type BacktestResult struct {
	Events []TriggeredEvent
}
type AlgoAlert interface {
	Name() string
	CheckAlert(conn *utils.Conn, sd *socket.SecurityData, timestamp time.Time) (bool, error)
}
type BacktestEngine struct {
	Conn    *utils.Conn
	Alerts  []AlgoAlert
	Results []BacktestResult
}
type SingleStockResult struct {
	SecurityID int
	Events     []TriggeredEvent
}

func BacktestSingleStock(conn *utils.Conn, securityId int, algo string, from, to time.Time) (SingleStockResult, error) {

}

func (be *BacktestEngine) Run() error {

}

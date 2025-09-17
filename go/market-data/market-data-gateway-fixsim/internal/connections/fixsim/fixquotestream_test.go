package fixsim

import (
	"context"
	"fmt"
	"github.com/ettec/otp-common/staticdata"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"github.com/ettec/open-trading-platform/go/market-data/market-data-gateway-fixsim/internal/fix/common"
	"github.com/ettec/open-trading-platform/go/market-data/market-data-gateway-fixsim/internal/fix/fix"
	md "github.com/ettec/open-trading-platform/go/market-data/market-data-gateway-fixsim/internal/fix/marketdata"
	"github.com/ettec/otp-common/model"
	"reflect"
	"strconv"
	"testing"
)

func Test_QuoteClone(t *testing.T) {
	quote := newClobQuote(2)
	quote.LastPrice = &model.Decimal64{Mantissa: 5, Exponent: 0}
	var lines []*model.ClobLine
	lines = append(lines, &model.ClobLine{Price: &model.Decimal64{Mantissa: 6, Exponent: 0},
		Size: &model.Decimal64{Mantissa: 30, Exponent: 0}})
	quote.Bids = lines

	bytes, err := proto.Marshal(quote)
	if err != nil {
		t.FailNow()
	}

	quoteCopy := &model.ClobQuote{}
	err = proto.Unmarshal(bytes, quoteCopy)
	if err != nil {
		t.FailNow()
	}

	lines = append(lines, &model.ClobLine{Price: &model.Decimal64{Mantissa: 8, Exponent: 0},
		Size: &model.Decimal64{Mantissa: 35, Exponent: 0}})

	quote.LastPrice = &model.Decimal64{Mantissa: 7, Exponent: 0}
	var lines2 []*model.ClobLine
	lines2 = append(lines, &model.ClobLine{Price: &model.Decimal64{Mantissa: 6, Exponent: 0},
		Size: &model.Decimal64{Mantissa: 30, Exponent: 0}})
	lines2 = append(lines, &model.ClobLine{Price: &model.Decimal64{Mantissa: 7, Exponent: 0},
		Size: &model.Decimal64{Mantissa: 40, Exponent: 0}})
	quote.Bids = lines2

	if !quoteCopy.LastPrice.Equal(&model.Decimal64{Mantissa: 5, Exponent: 0}) {
		t.FailNow()
	}

	if len(quoteCopy.Bids) != 1 {
		t.FailNow()
	}

}

type testFixClient struct {
	refreshChan   chan *md.MarketDataIncrementalRefresh
	subscribeChan chan string
}

func newTestMarketDataClient() (*testFixClient, error) {
	t := &testFixClient{
		refreshChan:   make(chan *md.MarketDataIncrementalRefresh, 100),
		subscribeChan: make(chan string, 100),
	}
	return t, nil
}

func (t *testFixClient) Chan() <-chan *md.MarketDataIncrementalRefresh {
	return t.refreshChan
}

func (t *testFixClient) Subscribe(symbol string) error {
	t.subscribeChan <- symbol
	return nil
}

func Test_quoteNormaliser_nilRefreshResetsAllQuote(t *testing.T) {
	fixClient, quoteStream := setupTestClient(t)

	quoteStream.Subscribe(1)
	quoteStream.Subscribe(2)
	quoteStream.Subscribe(3)

	entries := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_BID, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 10, 5, "A")}

	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries,
	}

	entries = []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_BID, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 10, 5, "B")}

	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries,
	}

	<-quoteStream.Chan()
	<-quoteStream.Chan()

	fixClient.refreshChan <- nil

	empt1 := <-quoteStream.Chan()
	if len(empt1.GetBids()) > 0 || len(empt1.GetOffers()) > 0 || (empt1.ListingId != 1 && empt1.ListingId != 2) || !empt1.StreamInterrupted {
		t.FailNow()
	}

	empt2 := <-quoteStream.Chan()
	if len(empt2.GetBids()) > 0 || len(empt2.GetOffers()) > 0 || (empt1.ListingId != 1 && empt1.ListingId != 2) || !empt2.StreamInterrupted {
		t.FailNow()
	}

}

func TestQuoteStreamInterruptedFlagResetOnUpdate(t *testing.T) {

	fixClient, quoteStream := setupTestClient(t)

	quoteStream.Subscribe(1)

	symbol := <-fixClient.subscribeChan
	assert.Equal(t, "A", symbol)

	entries := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_BID, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 10, 5, "A")}

	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries,
	}

	fixClient.refreshChan <- nil

	entries2 := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_OFFER, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 12, 5, "A")}

	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries2,
	}

	q := <-quoteStream.Chan()
	q = <-quoteStream.Chan()
	q = <-quoteStream.Chan()

	if q.StreamInterrupted {
		t.FailNow()
	}

}

func TestProcessingMDIncRefreshMessages(t *testing.T) {

	fixClient, quoteStream := setupTestClient(t)

	quoteStream.Subscribe(1)

	symbol := <-fixClient.subscribeChan
	assert.Equal(t, "A", symbol)

	entries := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_BID, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 10, 5, "A")}

	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries,
	}

	entries2 := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_OFFER, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 12, 5, "A")}

	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries2,
	}

	entries3 := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_OFFER, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 11, 2, "A")}
	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries3,
	}

	entries4 := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_TRADE, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 15, 10, "A")}
	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries4,
	}

	entries5 := []*md.MDIncGrp{getEntry(md.MDEntryTypeEnum_MD_ENTRY_TYPE_TRADE_VOLUME, md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW, 18, 120, "A")}
	fixClient.refreshChan <- &md.MarketDataIncrementalRefresh{
		MdIncGrp: entries5,
	}

	q := <-quoteStream.Chan()
	q = <-quoteStream.Chan()
	q = <-quoteStream.Chan()
	q = <-quoteStream.Chan()
	q = <-quoteStream.Chan()

	err := testEqualsBook(q, [5][4]int64{{5, 10, 11, 2}, {0, 0, 12, 5}}, 1)
	if err != nil {
		t.Errorf("Books not equal %v", err)
	}

	if q.LastQuantity.Mantissa != 10 || q.LastPrice.Mantissa != 15 {
		t.FailNow()
	}

	if q.TradedVolume.Mantissa != 120 {
		t.FailNow()
	}

}

func setupTestClient(t *testing.T) (*testFixClient, *FixQuoteStream) {
	tmd, err := newTestMarketDataClient()
	assert.NoError(t, err)

	listingIdToSym := map[int32]string{1: "A", 2: "B", 3: "C"}
	quoteStream, err := NewQuoteStreamFromFixClient(context.Background(), tmd, "testName", toLookupFunc(listingIdToSym), 100)
	assert.NoError(t, err)

	quoteStream.getListingResultChan = make(chan staticdata.ListingResult)

	return tmd, quoteStream
}

func toLookupFunc(listingIdToSym map[int32]string) func(ctx context.Context, listingId int32, onSymbol chan<- staticdata.ListingResult) {
	return func(ctx context.Context, listingId int32, onSymbol chan<- staticdata.ListingResult) {
		if sym, ok := listingIdToSym[listingId]; ok {
			onSymbol <- staticdata.ListingResult{Listing: &model.Listing{Id: listingId, MarketSymbol: sym}}
		}
	}
}

func testEqualsBook(quote *model.ClobQuote, book [5][4]int64, listingId int) error {

	if quote.ListingId != int32(listingId) {
		return fmt.Errorf("quote listing id and listing id are not the same")
	}

	var compare [5][4]int64

	for idx, line := range quote.Bids {
		compare[idx][0] = line.Size.Mantissa
		compare[idx][1] = line.Price.Mantissa
	}

	for idx, line := range quote.Offers {
		compare[idx][3] = line.Size.Mantissa
		compare[idx][2] = line.Price.Mantissa
	}

	if book != compare {
		return fmt.Errorf("expected book %v does not match book create from quote %v", book, compare)
	}

	return nil
}

var id = 0

func getNextId() string {
	id++
	return strconv.Itoa(id)
}

func getEntry(mt md.MDEntryTypeEnum, ma md.MDUpdateActionEnum, price int64, size int64, symbol string) *md.MDIncGrp {
	instrument := &common.Instrument{Symbol: symbol}
	entry := &md.MDIncGrp{
		MdEntryId:      getNextId(),
		MdEntryType:    mt,
		MdUpdateAction: ma,
		MdEntryPx:      &fix.Decimal64{Mantissa: price, Exponent: 0},
		MdEntrySize:    &fix.Decimal64{Mantissa: size, Exponent: 0},
		Instrument:     instrument,
	}
	return entry
}

func Test_updateAsksWithInserts(t *testing.T) {
	type args struct {
		asks   []*model.ClobLine
		update md.MDIncGrp
	}

	tests := []struct {
		name string
		args args
		want []*model.ClobLine
	}{

		{
			"insert ask into empty book",
			args{
				asks: []*model.ClobLine{},
				update: md.MDIncGrp{MdEntryId: "A", MdEntrySize: f64(20), MdEntryPx: f64(6),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{{EntryId: "A", Size: d64(20), Price: d64(6)}},
		},

		{
			"insert ask into middle of book",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(3),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "X", Size: d64(20), Price: d64(3)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(6)}},
		},

		{
			"insert ask at same price",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(4),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "X", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(6)}},
		},

		{
			"insert ask at top of book ",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(1),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "X", Size: d64(20), Price: d64(1)},
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(6)}},
		},

		{
			"insert ask at bottom of book ",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(8),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(6)},
				{EntryId: "X", Size: d64(20), Price: d64(8)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateClobLines(tt.args.asks, &tt.args.update, false); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateClobLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateAsksWithUpdates(t *testing.T) {
	type args struct {
		asks   []*model.ClobLine
		update md.MDIncGrp
	}

	tests := []struct {
		name string
		args args
		want []*model.ClobLine
	}{

		{
			"update ask quantity",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},
				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(10), MdEntryPx: f64(4),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "B", Size: d64(10), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(6)}},
		},

		{
			"update ask price - no order change",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},
				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(3),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "B", Size: d64(20), Price: d64(3)},
				{EntryId: "C", Size: d64(20), Price: d64(6)}},
		},

		{
			"update ask price down to same as other - order change",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(6),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "C", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(6)}},
		},

		{
			"update ask price up to same as other - order change",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(2),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "B", Size: d64(20), Price: d64(2)},
				{EntryId: "C", Size: d64(20), Price: d64(6)}},
		},

		{
			"update ask price up to top of book",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(1),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "B", Size: d64(20), Price: d64(1)},
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "C", Size: d64(20), Price: d64(6)}},
		},

		{
			"update ask price up to bottom of book",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(2)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(6)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(8),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(2)},
				{EntryId: "C", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(8)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateClobLines(tt.args.asks, &tt.args.update, false); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateClobLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateBidsWithInserts(t *testing.T) {
	type args struct {
		bids   []*model.ClobLine
		update md.MDIncGrp
	}

	tests := []struct {
		name string
		args args
		want []*model.ClobLine
	}{

		{
			"insert bid into empty book",
			args{
				bids: []*model.ClobLine{},
				update: md.MDIncGrp{MdEntryId: "A", MdEntrySize: f64(20), MdEntryPx: f64(6),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{{EntryId: "A", Size: d64(20), Price: d64(6)}},
		},

		{
			"insert bid into middle of book",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(3),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "X", Size: d64(20), Price: d64(3)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},

		{
			"insert bid into middle of book",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(3),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "X", Size: d64(20), Price: d64(3)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},

		{
			"insert bid at same price",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(4),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "X", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},

		{
			"insert bid at top of book ",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(8),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "X", Size: d64(20), Price: d64(8)},
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},

		{
			"insert bid at bottom of book ",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "X", MdEntrySize: f64(20), MdEntryPx: f64(1),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_NEW},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(2)},
				{EntryId: "X", Size: d64(20), Price: d64(1)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateClobLines(tt.args.bids, &tt.args.update, true); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateClobLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateBidsWithUpdates(t *testing.T) {
	type args struct {
		bids   []*model.ClobLine
		update md.MDIncGrp
	}

	tests := []struct {
		name string
		args args
		want []*model.ClobLine
	}{

		{
			"update bid quantity",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(10), MdEntryPx: f64(4),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(10), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},

		{
			"update bid price - no order change",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(10), MdEntryPx: f64(3),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(10), Price: d64(3)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},

		{
			"update bid price down to same as other - order change",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(3)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(3),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "C", Size: d64(20), Price: d64(3)},
				{EntryId: "B", Size: d64(20), Price: d64(3)}},
		},

		{
			"update bid price up to same as other - order change",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(3)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(6),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(6)},
				{EntryId: "C", Size: d64(20), Price: d64(3)}},
		},

		{
			"update bid price up to top of book",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(3)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(8),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "B", Size: d64(20), Price: d64(8)},
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "C", Size: d64(20), Price: d64(3)}},
		},

		{
			"update bid price up to bottom of book",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(3)}},

				update: md.MDIncGrp{MdEntryId: "B", MdEntrySize: f64(20), MdEntryPx: f64(2),
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_CHANGE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "C", Size: d64(20), Price: d64(3)},
				{EntryId: "B", Size: d64(20), Price: d64(2)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateClobLines(tt.args.bids, &tt.args.update, true); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateClobLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateBidsWithDelete(t *testing.T) {
	type args struct {
		bids   []*model.ClobLine
		update md.MDIncGrp
	}

	tests := []struct {
		name string
		args args
		want []*model.ClobLine
	}{

		{
			"delete from middle of book",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "B",
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_DELETE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},
		{
			"delete from top of book",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "A",
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_DELETE},
			},
			[]*model.ClobLine{

				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},
		{
			"delete from bottom of book",
			args{
				bids: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "C",
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_DELETE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(4)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateClobLines(tt.args.bids, &tt.args.update, true); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateClobLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateAsksWithDelete(t *testing.T) {
	type args struct {
		asks   []*model.ClobLine
		update md.MDIncGrp
	}

	tests := []struct {
		name string
		args args
		want []*model.ClobLine
	}{

		{
			"delete from middle of book",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "B",
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_DELETE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},
		{
			"delete from top of book",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "A",
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_DELETE},
			},
			[]*model.ClobLine{

				{EntryId: "B", Size: d64(20), Price: d64(4)},
				{EntryId: "C", Size: d64(20), Price: d64(2)}},
		},
		{
			"delete from bottom of book",
			args{
				asks: []*model.ClobLine{
					{EntryId: "A", Size: d64(20), Price: d64(6)},
					{EntryId: "B", Size: d64(20), Price: d64(4)},
					{EntryId: "C", Size: d64(20), Price: d64(2)}},
				update: md.MDIncGrp{MdEntryId: "C",
					MdUpdateAction: md.MDUpdateActionEnum_MD_UPDATE_ACTION_DELETE},
			},
			[]*model.ClobLine{
				{EntryId: "A", Size: d64(20), Price: d64(6)},
				{EntryId: "B", Size: d64(20), Price: d64(4)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateClobLines(tt.args.asks, &tt.args.update, false); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateClobLines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func d64(mantissa int) *model.Decimal64 {
	return &model.Decimal64{Mantissa: int64(mantissa), Exponent: 0}
}

func f64(mantissa int) *fix.Decimal64 {
	return &fix.Decimal64{Mantissa: int64(mantissa), Exponent: 0}
}

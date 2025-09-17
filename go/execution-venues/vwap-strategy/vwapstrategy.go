package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ettec/otp-common/model"
	"github.com/ettec/otp-common/strategy"
	"time"
)

type vwapParameters struct {
	UtcStartTimeSecs int64 `json:"utcStartTimeSecs"`
	UtcEndTimeSecs   int64 `json:"utcEndTimeSecs"`
	Buckets          int   `json:"buckets"`
}

func executeAsVwapStrategy(ctx context.Context, om *strategy.Strategy, buckets []bucket, listing *model.Listing) {

	go func() {

		if om.ParentOrder.GetTargetStatus() == model.OrderStatus_LIVE {
			err := om.ParentOrder.SetStatus(model.OrderStatus_LIVE)
			if err != nil {
				msg := fmt.Sprintf("failed to set order status, cancelling order:%v", err)
				om.Log.Error(msg)
				om.CancelChan <- msg
			}
		}

		om.Log.Info("order initialised", "buckets", buckets)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			done, err := om.CheckIfDone(ctx)
			if err != nil {
				msg := fmt.Sprintf("failed to check if done, cancelling order:%v", err)
				om.Log.Error(msg)
				om.CancelChan <- msg
			}

			if done {
				break
			}

			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				nowUtc := time.Now().Unix()
				shouldHaveSentQty := &model.Decimal64{}
				for i := 0; i < len(buckets); i++ {
					if buckets[i].utcStartTimeSecs <= nowUtc {
						shouldHaveSentQty.Add(&buckets[i].quantity)
					}
				}

				sentQty := &model.Decimal64{}
				sentQty.Add(om.ParentOrder.GetTradedQuantity())
				sentQty.Add(om.ParentOrder.GetExposedQuantity())

				if sentQty.LessThan(shouldHaveSentQty) {
					shouldHaveSentQty.Sub(sentQty)

					err := om.SendChildOrder(om.ParentOrder.Side, shouldHaveSentQty, om.ParentOrder.Price, listing.Id,
						listing.Market.Mic, "")
					if err != nil {
						om.CancelChan <- fmt.Sprintf("failed to send child order:%v", err)
					}
				}

			case errMsg := <-om.CancelChan:
				if errMsg != "" {
					om.ParentOrder.ErrorMessage = errMsg
				}
				err := om.CancelChildOrdersAndStrategyOrder()
				if err != nil {
					om.Log.Error("failed to cancel order", "error", err)
				}
			case co, ok := <-om.ChildOrderUpdateChan:
				err = om.OnChildOrderUpdate(ok, co)
				if err != nil {
					om.Log.Error("failed to process child order update", "error", err)
				}
			}

		}

	}()
}

func getBucketsFromParamsString(vwapParamsJson string, quantity model.Decimal64, listing *model.Listing) ([]bucket, error) {
	vwapParameters := &vwapParameters{}
	err := json.Unmarshal([]byte(vwapParamsJson), vwapParameters)
	if err != nil {
		return nil, err
	}

	numBuckets := vwapParameters.Buckets
	if numBuckets == 0 {
		numBuckets = 10
	}

	buckets := getBuckets(listing, vwapParameters.UtcStartTimeSecs, vwapParameters.UtcEndTimeSecs, numBuckets, quantity)
	return buckets, nil
}

type bucket struct {
	quantity         model.Decimal64
	utcStartTimeSecs int64
	utcEndTimeSecs   int64
}

func getBuckets(listing *model.Listing, utcStartTimeSecs int64, utcEndTimeSecs int64, buckets int, quantity model.Decimal64) (result []bucket) {
	// need historical traded volume data, for now use a TWAP profile
	bucketInterval := (utcEndTimeSecs - utcStartTimeSecs) / int64(buckets)

	fBuckets := float64(buckets)
	fQuantity := quantity.ToFloat()
	bucketQnt := fQuantity / fBuckets

	startTime := utcStartTimeSecs
	endTime := startTime + bucketInterval

	for i := 0; i < buckets; i++ {
		bucket := bucket{
			quantity:         *listing.RoundToLotSize(bucketQnt),
			utcStartTimeSecs: startTime,
			utcEndTimeSecs:   endTime,
		}
		result = append(result, bucket)

		startTime = endTime
		endTime = endTime + bucketInterval
	}

	var totalQnt model.Decimal64
	for _, bucket := range result {
		totalQnt.Add(&bucket.quantity)
	}

	quantity.Sub(&totalQnt)
	if result != nil {
		result[len(result)-1].quantity.Add(&quantity)
	}

	return result
}

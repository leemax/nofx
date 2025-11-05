package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Data 市场数据结构
type Data struct {
	Symbol            string
	CurrentPrice      float64
	PriceChange1h     float64 // 1小时价格变化百分比
	PriceChange4h     float64 // 4小时价格变化百分比
	OpenInterest      *OIData
	FundingRate       float64
	EMA50_15m         float64 // 15M EMA-50
	IntradaySeries    *IntradayData   // 3m
	FifteenMinSeries  *FifteenMinData // 15m
	OneHourContext    *OneHourData    // 1h
	FourHourContext   *FourHourData   // 4h
	DailyContext      *DailyData      // 1d
}

// OIData Open Interest数据
type OIData struct {
	Latest  float64
	Average float64
}

// IntradayData 日内数据(3分钟间隔)
type IntradayData struct {
	CurrentEMA20 float64
	CurrentMACD  float64
	CurrentRSI7  float64
	MidPrices    []float64
	EMA20Values  []float64
	MACDValues   []float64
	RSI7Values   []float64
	RSI14Values  []float64
	BBandsUpper  []float64 // 3M 布林带上轨 (20-period, 2.0 stdDev)
	BBandsLower  []float64 // 3M 布林带下轨 (20-period, 2.0 stdDev)
	ATR14        float64   // 3M ATR (14周期)
	ADXValues    []float64 // 3M ADX(14)
}

// FifteenMinData 15分钟数据
type FifteenMinData struct {
	RSI14Values      []float64 // 最近10个RSI(14)值
	MACDLineValues   []float64 // 最近10个MACD线值
	MACDSignalValues []float64 // 最近10个MACD信号线值
}

// OneHourData 1小时数据
type OneHourData struct {
	EMA50 float64
	ATR14 float64
}

// FourHourData 4小时数据
type FourHourData struct {
	EMA20         float64
	EMA50         float64
	ADX14         float64
	ATR14         float64
	NextResistance float64
	NextSupport    float64
	CurrentVolume float64
	AverageVolume float64
	MACDValues    []float64
	RSI14Values   []float64
}

// DailyData 日线数据
type DailyData struct {
	EMA50 float64
}

// Kline K线数据
type Kline struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// Get 获取指定代币的市场数据
func Get(symbol string) (*Data, error) {
	// 标准化symbol
	symbol = Normalize(symbol)

	// 并发获取K线数据
	var klines3m, klines15m, klines1h, klines4h, klines1d []Kline
	var err3m, err15m, err1h, err4h, err1d error
	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		klines3m, err3m = getKlines(symbol, "3m", 100) // 需要更多数据用于计算序列
	}()
	go func() {
		defer wg.Done()
		klines15m, err15m = getKlines(symbol, "15m", 100) // 需要更多数据用于计算序列
	}()
	go func() {
		defer wg.Done()
		klines1h, err1h = getKlines(symbol, "1h", 100)
	}()
	go func() {
		defer wg.Done()
		klines4h, err4h = getKlines(symbol, "4h", 100)
	}()
	go func() {
		defer wg.Done()
		klines1d, err1d = getKlines(symbol, "1d", 100)
	}()

	wg.Wait()

	if err3m != nil {
		return nil, fmt.Errorf("获取3分钟K线失败: %v", err3m)
	}
	if err15m != nil {
		return nil, fmt.Errorf("获取15分钟K线失败: %v", err15m)
	}
	if err1h != nil {
		return nil, fmt.Errorf("获取1小时K线失败: %v", err1h)
	}
	if err4h != nil {
		return nil, fmt.Errorf("获取4小时K线失败: %v", err4h)
	}
	if err1d != nil {
		return nil, fmt.Errorf("获取日线K线失败: %v", err1d)
	}

	// 计算当前价格
	currentPrice := klines3m[len(klines3m)-1].Close

	// 计算价格变化百分比
	priceChange1h := 0.0
	if len(klines1h) >= 2 {
		price1hAgo := klines1h[len(klines1h)-2].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 获取OI和Funding Rate
	oiData, _ := getOpenInterestData(symbol)
	fundingRate, _ := getFundingRate(symbol)

	// 并发计算所有时间周期的数据
	var intradayData *IntradayData
	var fifteenMinData *FifteenMinData
	var oneHourData *OneHourData
	var fourHourData *FourHourData
	var dailyData *DailyData

	wg.Add(5)
	go func() {
		defer wg.Done()
		intradayData = calculateIntradaySeries(klines3m)
	}()
	go func() {
		defer wg.Done()
		fifteenMinData = calculateFifteenMinSeries(klines15m)
	}()
	go func() {
		defer wg.Done()
		oneHourData = calculateOneHourContext(klines1h)
	}()
	go func() {
		defer wg.Done()
		fourHourData = calculateFourHourContext(klines4h)
	}()
	go func() {
		defer wg.Done()
		dailyData = calculateDailyContext(klines1d)
	}()
	wg.Wait()

	return &Data{
		Symbol:           symbol,
		CurrentPrice:     currentPrice,
		PriceChange1h:    priceChange1h,
		PriceChange4h:    priceChange4h,
		OpenInterest:     oiData,
		FundingRate:      fundingRate,
		IntradaySeries:   intradayData,
		FifteenMinSeries: fifteenMinData,
		OneHourContext:   oneHourData,
		FourHourContext:  fourHourData,
		DailyContext:     dailyData,
	}, nil
}

// getKlines 从Binance获取K线数据
func getKlines(symbol, interval string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	klines := make([]Kline, len(rawData))
	for i, item := range rawData {
		openTime := int64(item[0].(float64))
		open, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])
		closeTime := int64(item[6].(float64))

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	macdLine, _ := calculateMACDSeries(klines, 12, 26, 9)
	if len(macdLine) == 0 {
		return 0
	}

	return macdLine[len(macdLine)-1]
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	rsiSeries := calculateRSISeries(klines, period)
	if len(rsiSeries) == 0 {
		return 0
	}
	return rsiSeries[len(rsiSeries)-1]
}

// calculateEMASeries 计算EMA序列
func calculateEMASeries(klines []Kline, period int) []float64 {
	if len(klines) < period {
		return nil
	}

emas := make([]float64, len(klines))
	multiplier := 2.0 / float64(period+1)

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	emas[period-1] = sum / float64(period)

	// 计算后续EMA
	for i := period; i < len(klines); i++ {
		emas[i] = (klines[i].Close-emas[i-1])*multiplier + emas[i-1]
	}

	return emas
}

// calculateRSISeries 计算RSI序列
func calculateRSISeries(klines []Kline, period int) []float64 {
	if len(klines) <= period {
		return nil
	}

	rsis := make([]float64, len(klines))
	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss > 0 {
		rs := avgGain / avgLoss
		rsis[period] = 100 - (100 / (1 + rs))
	} else {
		rsis[period] = 100
	}

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		currentGain := 0.0
		currentLoss := 0.0
		if change > 0 {
			currentGain = change
		} else {
			currentLoss = -change
		}

		avgGain = (avgGain*float64(period-1) + currentGain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + currentLoss) / float64(period)

		if avgLoss > 0 {
			rs := avgGain / avgLoss
			rsis[i] = 100 - (100 / (1 + rs))
		} else {
			rsis[i] = 100
		}
	}

	return rsis
}

// calculateMACDSeries 计算MACD线和信号线序列
func calculateMACDSeries(klines []Kline, fastPeriod, slowPeriod, signalPeriod int) ([]float64, []float64) {
	if len(klines) < slowPeriod {
		return nil, nil
	}

	emaFast := calculateEMASeries(klines, fastPeriod)
	emaSlow := calculateEMASeries(klines, slowPeriod)

	macdLine := make([]float64, len(klines))
	for i := slowPeriod - 1; i < len(klines); i++ {
		macdLine[i] = emaFast[i] - emaSlow[i]
	}

	// 将MACD线作为输入来计算信号线
	macdKlines := make([]Kline, len(macdLine))
	for i, v := range macdLine {
		macdKlines[i] = Kline{Close: v}
	}

	signalLine := calculateEMASeries(macdKlines, signalPeriod)

	return macdLine, signalLine
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateBollingerBands 计算布林带
func calculateBollingerBands(klines []Kline, period int, numStdDev float64) (upper []float64, lower []float64) {
	if len(klines) < period {
		return nil, nil
	}

	upper = make([]float64, len(klines)-period+1)
	lower = make([]float64, len(klines)-period+1)

	for i := period - 1; i < len(klines); i++ {
		// 获取当前窗口的收盘价
		window := make([]float64, period)
		for j := 0; j < period; j++ {
			window[j] = klines[i-period+1+j].Close
		}

		// 计算SMA
		sum := 0.0
		for _, price := range window {
			sum += price
		}
		sma := sum / float64(period)

		// 计算标准差
		sumSqDiff := 0.0
		for _, price := range window {
			sumSqDiff += (price - sma) * (price - sma)
		}
		stdDev := math.Sqrt(sumSqDiff / float64(period))

		// 计算上轨和下轨
		upper[i-period+1] = sma + (stdDev * numStdDev)
		lower[i-period+1] = sma - (stdDev * numStdDev)
	}

	return upper, lower
}

// calculateADX 计算ADX (Average Directional Index)
func calculateADX(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0.0
	}

	plusDM := make([]float64, len(klines))
	minusDM := make([]float64, len(klines))
	trueRange := make([]float64, len(klines))

	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevHigh := klines[i-1].High
		prevLow := klines[i-1].Low

		// 计算True Range (TR)
		tr1 := high - low
		tr2 := math.Abs(high - prevHigh)
		tr3 := math.Abs(low - prevLow)
		trueRange[i] = math.Max(tr1, math.Max(tr2, tr3))

		// 计算Directional Movement (DM)
		moveUp := high - prevHigh
		moveDown := prevLow - low

		if moveUp > moveDown && moveUp > 0 {
			plusDM[i] = moveUp
		} else {
			plusDM[i] = 0
		}

		if moveDown > moveUp && moveDown > 0 {
			minusDM[i] = moveDown
		} else {
			minusDM[i] = 0
		}
	}

	// 计算平滑的TR, +DM, -DM
	smoothTR := make([]float64, len(klines))
	smoothPlusDM := make([]float64, len(klines))
	smoothMinusDM := make([]float64, len(klines))

	// 初始平滑值 (SMA)
	sumTR := 0.0
	sumPlusDM := 0.0
	sumMinusDM := 0.0
	for i := 1; i <= period; i++ {
		sumTR += trueRange[i]
		sumPlusDM += plusDM[i]
		sumMinusDM += minusDM[i]
	}
	smoothTR[period] = sumTR
	smoothPlusDM[period] = sumPlusDM
	smoothMinusDM[period] = sumMinusDM

	// 后续平滑值 (Wilder's smoothing)
	for i := period + 1; i < len(klines); i++ {
		smoothTR[i] = smoothTR[i-1] - (smoothTR[i-1] / float64(period)) + trueRange[i]
		smoothPlusDM[i] = smoothPlusDM[i-1] - (smoothPlusDM[i-1] / float64(period)) + plusDM[i]
		smoothMinusDM[i] = smoothMinusDM[i-1] - (smoothMinusDM[i-1] / float64(period)) + minusDM[i]
	}

	// 计算DI
	plusDI := make([]float64, len(klines))
	minusDI := make([]float64, len(klines))
	dx := make([]float64, len(klines))

	for i := period; i < len(klines); i++ {
		if smoothTR[i] == 0 {
			plusDI[i] = 0
			minusDI[i] = 0
		} else {
			plusDI[i] = (smoothPlusDM[i] / smoothTR[i]) * 100
			minusDI[i] = (smoothMinusDM[i] / smoothTR[i]) * 100
		}

		diSum := plusDI[i] + minusDI[i]
		if diSum == 0 {
			dx[i] = 0
		} else {
			dx[i] = (math.Abs(plusDI[i]-minusDI[i]) / diSum) * 100
		}
	}

	// 计算ADX (平滑DX)
	adx := 0.0
	// 初始ADX (SMA)
	sumDX := 0.0
	for i := period; i < period*2; i++ {
		sumDX += dx[i]
	}
	if period > 0 {
		adx = sumDX / float64(period)
	}

	// 后续ADX (Wilder's smoothing)
	for i := period*2; i < len(klines); i++ {
		adx = (adx*float64(period-1) + dx[i]) / float64(period)
	}

	return adx
}

// calculateIntradaySeries 计算日内系列数据 (3m)
func calculateIntradaySeries(klines []Kline) *IntradayData {
	if len(klines) == 0 {
		return &IntradayData{}
	}
	return &IntradayData{
		CurrentEMA20: calculateEMA(klines, 20),
		CurrentMACD:  calculateMACD(klines),
		CurrentRSI7:  calculateRSI(klines, 7),
		ATR14:        calculateATR(klines, 14),
	}
}

// calculateFifteenMinSeries 计算15分钟序列数据
func calculateFifteenMinSeries(klines []Kline) *FifteenMinData {
	data := &FifteenMinData{}
	if len(klines) < 26 { // MACD(12,26,9) 需要的最少数据
		return data
	}

	// RSI (14)
	rsi14Series := calculateRSISeries(klines, 14)

	// MACD (12, 26, 9)
	macdLine, macdSignal := calculateMACDSeries(klines, 12, 26, 9)

	// 获取最近10个值
	data.RSI14Values = getLastN(rsi14Series, 10)
	data.MACDLineValues = getLastN(macdLine, 10)
	data.MACDSignalValues = getLastN(macdSignal, 10)

	return data
}

// calculateOneHourContext 计算1小时数据
func calculateOneHourContext(klines []Kline) *OneHourData {
	if len(klines) == 0 {
		return &OneHourData{}
	}
	return &OneHourData{
		EMA50: calculateEMA(klines, 50),
		ATR14: calculateATR(klines, 14),
	}
}

// calculateFourHourContext 计算4小时数据
func calculateFourHourContext(klines []Kline) *FourHourData {
	if len(klines) == 0 {
		return &FourHourData{}
	}

	support, resistance := calculateSupportResistance(klines, 20) // 使用最近20根K线计算支撑阻力

	return &FourHourData{
		EMA20:          calculateEMA(klines, 20),
		EMA50:          calculateEMA(klines, 50),
		ADX14:          calculateADX(klines, 14),
		ATR14:          calculateATR(klines, 14),
		NextSupport:    support,
		NextResistance: resistance,
	}
}

// calculateDailyContext 计算日线数据
func calculateDailyContext(klines []Kline) *DailyData {
	if len(klines) == 0 {
		return &DailyData{}
	}
	return &DailyData{
		EMA50: calculateEMA(klines, 50),
	}
}

// calculateSupportResistance 计算支撑和阻力位 (基于最近N个周期的高低点)
func calculateSupportResistance(klines []Kline, period int) (support float64, resistance float64) {
	if len(klines) == 0 {
		return 0, 0
	}

	// 确定要分析的K线范围
	startIndex := len(klines) - period
	if startIndex < 0 {
		startIndex = 0
	}
	analysisKlines := klines[startIndex:]

	if len(analysisKlines) == 0 {
		return 0, 0
	}

	// 寻找这个范围内的最高价和最低价
	highestHigh := analysisKlines[0].High
	lowestLow := analysisKlines[0].Low

	for _, k := range analysisKlines {
		if k.High > highestHigh {
			highestHigh = k.High
		}
		if k.Low < lowestLow {
			lowestLow = k.Low
		}
	}

	return lowestLow, highestHigh
}

// getLastN 获取切片的最后N个元素
func getLastN(slice []float64, n int) []float64 {
	if len(slice) == 0 {
		return nil
	}
	start := len(slice) - n
	if start < 0 {
		start = 0
	}
	return slice[start:]
}

// getOpenInterestData 获取OI数据
func getOpenInterestData(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // 近似平均值
	}, nil
}

// getFundingRate 获取资金费率
func getFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("### %s Market Data\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("**Current Price**: `%.4f`\n", data.CurrentPrice))
	sb.WriteString(fmt.Sprintf("**Price Change**: 1h: `%.2f%%` | 4h: `%.2f%%`\n", data.PriceChange1h, data.PriceChange4h))

	// 1D Data
	if data.DailyContext != nil {
		sb.WriteString("\n**Daily (1D) Data:**\n")
		sb.WriteString(fmt.Sprintf("- **1D_EMA_50**: `%.4f`\n", data.DailyContext.EMA50))
	}

	// 4H Data
	if data.FourHourContext != nil {
		sb.WriteString("\n**4-Hour (4H) Data:**\n")
		sb.WriteString(fmt.Sprintf("- **4H_EMA_20**: `%.4f`\n", data.FourHourContext.EMA20))
		sb.WriteString(fmt.Sprintf("- **4H_EMA_50**: `%.4f`\n", data.FourHourContext.EMA50))
		sb.WriteString(fmt.Sprintf("- **4H_ADX_14**: `%.2f`\n", data.FourHourContext.ADX14))
		sb.WriteString(fmt.Sprintf("- **4H_ATR_14**: `%.4f`\n", data.FourHourContext.ATR14))
		sb.WriteString(fmt.Sprintf("- **4H_Next_Support**: `%.4f`\n", data.FourHourContext.NextSupport))
		sb.WriteString(fmt.Sprintf("- **4H_Next_Resistance**: `%.4f`\n", data.FourHourContext.NextResistance))
	}

	// 1H Data
	if data.OneHourContext != nil {
		sb.WriteString("\n**1-Hour (1H) Data:**\n")
		sb.WriteString(fmt.Sprintf("- **1H_EMA_50**: `%.4f`\n", data.OneHourContext.EMA50))
		sb.WriteString(fmt.Sprintf("- **1H_ATR_14**: `%.4f`\n", data.OneHourContext.ATR14))
	}

	// 15M Data
	if data.FifteenMinSeries != nil {
		sb.WriteString("\n**15-Minute (15M) Data (Trigger Timeframe):**\n")
		if data.FifteenMinSeries.RSI14Values != nil {
			sb.WriteString(fmt.Sprintf("- **15M_RSI_14_series (last 10)**: `%s`\n", formatFloatSlice(data.FifteenMinSeries.RSI14Values, 4)))
		}
		if data.FifteenMinSeries.MACDLineValues != nil {
			sb.WriteString(fmt.Sprintf("- **15M_MACD_line_series (last 10)**: `%s`\n", formatFloatSlice(data.FifteenMinSeries.MACDLineValues, 4)))
		}
		if data.FifteenMinSeries.MACDSignalValues != nil {
			sb.WriteString(fmt.Sprintf("- **15M_MACD_signal_series (last 10)**: `%s`\n", formatFloatSlice(data.FifteenMinSeries.MACDSignalValues, 4)))
		}
	}

	return sb.String()
}

// formatFloatSlice 格式化float64切片为字符串
func formatFloatSlice(values []float64, precision int) string {
	if values == nil {
		return "[]"
	}
	strValues := make([]string, len(values))
	format := fmt.Sprintf("%%.%df", precision)
	for i, v := range values {
		strValues[i] = fmt.Sprintf(format, v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// +build !nometrics

package gopb

func NewNodeMetrics(sid uint64, metrics []byte) *Msg {
	return newMsg(MSG_METRICS, "", sid, true, &Metrics{Kind: METRICS_NODE, Data: metrics})
}

func NewMetricsConfigured(sid uint64) *Msg {
	return newMsg(MSG_METRICS, "", sid, true, &Metrics{Kind: METRICS_COLLECTION_CONFIGURED})
}

func NewContainerMetrics(sid uint64, metrics []byte) *Msg {
	return newMsg(MSG_METRICS, "", sid, true, &Metrics{Kind: METRICS_CONTAINER, Data: metrics})
}

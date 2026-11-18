ALTER TABLE metric_histograms_1d ADD COLUMN IF NOT EXISTS weight_squares SimpleAggregateFunction(sum, Float64) DEFAULT cnt * 1000

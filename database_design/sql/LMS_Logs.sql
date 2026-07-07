CREATE TABLE LMS.LMS_Logs
(
    `Log_ID` String,
    `Timestamp` DateTime DEFAULT now(),
    `Level` String DEFAULT '',
    `Host` String DEFAULT '',
    `Source_Type` String DEFAULT '',
    `Message` String DEFAULT '',
    `Tags` Map(String, String) DEFAULT map(),
    `Collector_ID` String DEFAULT ''
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(Timestamp)
ORDER BY (Timestamp, Level)
TTL Timestamp + toIntervalDay(7) TO VOLUME 'warm_volume', Timestamp + toIntervalDay(30) TO VOLUME 'cold_volume', Timestamp + toIntervalDay(180)
SETTINGS storage_policy = 'hot_warm_cold', index_granularity = 8192

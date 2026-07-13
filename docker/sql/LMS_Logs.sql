CREATE TABLE IF NOT EXISTS LMS.LMS_Logs
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
SETTINGS index_granularity = 8192

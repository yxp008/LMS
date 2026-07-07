CREATE TABLE LMS.LMS_Collectors
(
    `Collector_ID` String,
    `Name` String,
    `Status` String DEFAULT '1'
)
ENGINE = MergeTree
ORDER BY (Collector_ID)
SETTINGS index_granularity = 8192

CREATE TABLE LMS.LMS_Collectors
(
    `Collector_ID` String,
    `Name` String,
    `Status` String DEFAULT '1',
    `Address` String DEFAULT '',
    `Source_Types` String DEFAULT '[]',
    `Source_Host` String DEFAULT '',
    `Last_Seen` DateTime DEFAULT now()
)
ENGINE = MergeTree
ORDER BY (Collector_ID)
SETTINGS index_granularity = 8192

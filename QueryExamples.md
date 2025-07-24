# ArangoDB Datasource Query Examples

This document provides examples of various queries you can use with the ArangoDB datasource.

## Setup Example Data

First, let's create some example collections and data:

```javascript
// Create a metrics collection
db._create("metrics");

// Insert sample time series data
var startTime = new Date('2024-01-01').getTime();
for (let i = 0; i < 1000; i++) {
  db.metrics.insert({
    timestamp: startTime + (i * 60000), // Every minute
    value: Math.random() * 100,
    sensor_id: "sensor_" + (i % 5),
    location: ["room_a", "room_b", "room_c"][i % 3],
    temperature: 20 + Math.random() * 15,
    humidity: 40 + Math.random() * 20
  });
}

// Create a users collection
db._create("users");

// Insert user data
db.users.insert([
  { _key: "user1", name: "John Doe", email: "john@example.com", created_at: new Date('2024-01-01').getTime() },
  { _key: "user2", name: "Jane Smith", email: "jane@example.com", created_at: new Date('2024-01-02').getTime() },
  { _key: "user3", name: "Bob Wilson", email: "bob@example.com", created_at: new Date('2024-01-03').getTime() }
]);

// Create events collection
db._create("events");

// Insert event data
var events = [];
for (let i = 0; i < 500; i++) {
  events.push({
    user_id: "user" + ((i % 3) + 1),
    event_type: ["login", "logout", "purchase", "view"][i % 4],
    timestamp: startTime + (i * 120000), // Every 2 minutes
    metadata: {
      ip_address: "192.168.1." + (i % 255),
      user_agent: "Browser " + (i % 10)
    }
  });
}
db.events.insert(events);
```

## Basic Queries

### 1. Simple Time Series Query

**Collection-based Query:**
- Collection: `metrics`
- Time Field: `timestamp`
- Value Field: `value`
- Sort By: `timestamp`
- Sort Order: `ASC`

**Equivalent AQL:**
```aql
FOR doc IN metrics
  FILTER doc.timestamp >= $__timeFrom AND doc.timestamp <= $__timeTo
  SORT doc.timestamp ASC
  RETURN doc
```

### 2. Filtered Time Series

**AQL Query:**
```aql
FOR doc IN metrics
  FILTER $__timeFilter(doc.timestamp)
  FILTER doc.sensor_id == "sensor_1"
  SORT doc.timestamp ASC
  RETURN {
    time: doc.timestamp,
    value: doc.value,
    location: doc.location
  }
```

### 3. Aggregated Data by Time Buckets

**AQL Query:**
```aql
FOR doc IN metrics
  FILTER $__timeFilter(doc.timestamp)
  COLLECT timeGroup = FLOOR(doc.timestamp / 300000) * 300000 INTO g
  RETURN {
    time: timeGroup,
    value: AVG(g[*].doc.value),
    count: LENGTH(g)
  }
```

## Advanced Queries

### 4. Multi-Collection Join Query

```aql
FOR user IN users
  FOR event IN events
    FILTER event.user_id == user._key
    FILTER $__timeFilter(event.timestamp)
    RETURN {
      time: event.timestamp,
      user: user.name,
      event_type: event.event_type,
      value: 1
    }
```

### 5. Grouped Metrics by Location

**Collection-based Query:**
- Collection: `metrics`
- Time Field: `timestamp`
- Value Field: `value`
- Group By: `location`
- Aggregation: `AVG`

**Equivalent AQL:**
```aql
FOR doc IN metrics
  FILTER $__timeFilter(doc.timestamp)
  COLLECT location = doc.location INTO g
  RETURN {
    location: location,
    avg_value: AVG(g[*].doc.value),
    count: LENGTH(g)
  }
```

### 6. Top N Query

```aql
FOR doc IN metrics
  FILTER $__timeFilter(doc.timestamp)
  COLLECT sensor = doc.sensor_id INTO g
  SORT AVG(g[*].doc.value) DESC
  LIMIT 5
  RETURN {
    sensor: sensor,
    avg_value: AVG(g[*].doc.value),
    max_value: MAX(g[*].doc.value),
    count: LENGTH(g)
  }
```

### 7. Event Count Over Time

```aql
FOR event IN events
  FILTER $__timeFilter(event.timestamp)
  COLLECT timeGroup = FLOOR(event.timestamp / 3600000) * 3600000,
          eventType = event.event_type INTO g
  RETURN {
    time: timeGroup,
    event_type: eventType,
    count: LENGTH(g)
  }
```

### 8. Moving Average Query

```aql
LET timeWindows = (
  FOR doc IN metrics
    FILTER $__timeFilter(doc.timestamp)
    SORT doc.timestamp
    RETURN doc
)

FOR i IN 0..(LENGTH(timeWindows) - 1)
  LET currentDoc = timeWindows[i]
  LET windowStart = MAX([0, i - 4])
  LET windowDocs = SLICE(timeWindows, windowStart, 5)
  RETURN {
    time: currentDoc.timestamp,
    value: currentDoc.value,
    moving_avg: AVG(windowDocs[*].value)
  }
```

## Performance Optimization Examples

### 9. Indexed Query with Compound Filter

```aql
// Ensure you have an index: db.metrics.ensureIndex({type: "persistent", fields: ["location", "timestamp"]})
FOR doc IN metrics
  FILTER doc.location == "room_a"
  FILTER $__timeFilter(doc.timestamp)
  SORT doc.timestamp ASC
  RETURN {
    time: doc.timestamp,
    value: doc.value,
    temperature: doc.temperature
  }
```

### 10. Efficient Aggregation with Pre-filtering

```aql
FOR doc IN metrics
  FILTER doc.timestamp >= $__timeFrom AND doc.timestamp <= $__timeTo
  FILTER doc.value > 50  // Pre-filter before aggregation
  COLLECT location = doc.location INTO g
  RETURN {
    location: location,
    high_value_avg: AVG(g[*].doc.value),
    high_value_count: LENGTH(g)
  }
```

## Template Variable Queries

### Collections Variable
Query: `collections`

### Fields Variable (for a specific collection)
Query: `fields:metrics`

### Locations Variable
```aql
FOR doc IN metrics
  COLLECT location = doc.location
  RETURN location
```

### Sensor IDs Variable
```aql
FOR doc IN metrics
  COLLECT sensor = doc.sensor_id
  RETURN sensor
```

## Real-world Use Cases

### 11. System Monitoring Dashboard

```aql
FOR doc IN system_metrics
  FILTER $__timeFilter(doc.timestamp)
  FILTER doc.metric_type == "cpu_usage"
  COLLECT timeGroup = FLOOR(doc.timestamp / 300000) * 300000,
          hostname = doc.hostname INTO g
  RETURN {
    time: timeGroup,
    hostname: hostname,
    avg_cpu: AVG(g[*].doc.value),
    max_cpu: MAX(g[*].doc.value)
  }
```

### 12. Business Analytics

```aql
FOR order IN orders
  FILTER $__timeFilter(order.created_at)
  COLLECT timeGroup = FLOOR(order.created_at / 3600000) * 3600000 INTO g
  RETURN {
    time: timeGroup,
    total_orders: LENGTH(g),
    total_revenue: SUM(g[*].order.amount),
    avg_order_value: AVG(g[*].order.amount)
  }
```

### 13. Error Rate Monitoring

```aql
FOR log IN application_logs
  FILTER $__timeFilter(log.timestamp)
  COLLECT timeGroup = FLOOR(log.timestamp / 300000) * 300000,
          level = log.level INTO g
  RETURN {
    time: timeGroup,
    level: level,
    count: LENGTH(g)
  }
```

## Tips for Writing Efficient Queries

1. **Use Indexes**: Always create appropriate indexes for fields used in FILTER clauses
2. **Filter Early**: Apply filters as early as possible in the query
3. **Limit Results**: Use LIMIT to avoid loading too much data
4. **Use Compound Indexes**: For multi-field filters, create compound indexes
5. **Avoid Scanning Large Collections**: Use specific filters rather than broad scans
6. **Pre-aggregate When Possible**: Consider pre-aggregating data at write time for frequently accessed metrics

## Common Patterns

### Time Bucketing
```aql
COLLECT timeGroup = FLOOR(doc.timestamp / interval) * interval
```

### Conditional Aggregation
```aql
RETURN {
  total: LENGTH(g),
  errors: LENGTH(g[* FILTER CURRENT.doc.status == "error"]),
  success: LENGTH(g[* FILTER CURRENT.doc.status == "success"])
}
```

### Percentile Calculation
```aql
LET sorted = (FOR doc IN collection SORT doc.value RETURN doc.value)
RETURN {
  p50: PERCENTILE(sorted, 50),
  p95: PERCENTILE(sorted, 95),
  p99: PERCENTILE(sorted, 99)
}
```

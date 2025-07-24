# Grafana ArangoDB Datasource

A comprehensive Grafana datasource plugin for ArangoDB that supports custom AQL queries and multiple collections.

## Features

- **Custom AQL Queries**: Write and execute custom AQL (ArangoDB Query Language) queries with shorthand syntax support
- **Collection-based Queries**: Build queries using a visual interface for specific collections
- **Advanced Data Flattening**: Intelligent flattening of nested objects and arrays with user control
- **Multiple Collection Support**: Query across different collections in your ArangoDB database
- **Time Series Support**: Built-in support for time-based filtering and visualization
- **Aggregation Functions**: Support for COUNT, SUM, AVG, MIN, MAX aggregations
- **Variable Interpolation**: Support for Grafana template variables and time range variables
- **Field Discovery**: Automatic discovery of collection fields for query building
- **Schema Heterogeneity**: Handles documents with different schemas in the same collection
- **Array Support**: Full support for array flattening with index notation
- **Auto-prefixing**: Automatic `doc.` prefixing in filters for improved usability

## Installation

### Option 1: Using Build Scripts

#### For Linux/Unix Systems:
```bash
# Clone the repository
git clone https://github.com/your-username/grafana-arangodb-datasource.git
cd grafana-arangodb-datasource

# Build for Linux platforms (x64 and ARM64)
chmod +x build.sh
./build.sh

# Or use Makefile
make linux        # Build for both Linux x64 and ARM64
make linux-amd64  # Build for Linux x64 only
make linux-arm64  # Build for Linux ARM64 only
```

#### For Windows Systems:
```powershell
# Clone the repository
git clone https://github.com/your-username/grafana-arangodb-datasource.git
cd grafana-arangodb-datasource

# Run PowerShell build script
.\setup.ps1
# Follow the prompts to select target platforms:
# 1. Linux x64 only
# 2. Linux ARM64 only  
# 3. Both Linux x64 and ARM64 (default)
# 4. Windows x64 (for development)
# 5. All platforms
```

### Option 2: Manual Build

1. Install dependencies:
   ```bash
   npm install
   cd pkg && go mod tidy
   ```

2. Build frontend:
   ```bash
   npm run build
   ```

3. Build backend for your target platform:
   ```bash
   # For Linux x64
   cd pkg && GOOS=linux GOARCH=amd64 go build -o ../dist/gpx_arangodb-datasource_linux_amd64 .
   
   # For Linux ARM64
   cd pkg && GOOS=linux GOARCH=arm64 go build -o ../dist/gpx_arangodb-datasource_linux_arm64 .
   ```

4. Copy files to Grafana plugins directory:
   ```bash
   cp -r dist /var/lib/grafana/plugins/arangodb-datasource/
   ```

5. Restart Grafana

## Configuration

### Basic Settings

- **URL**: The ArangoDB server URL (e.g., `http://localhost:8529`)
- **Database**: The database name to connect to (default: `_system`)
- **Username**: Optional username for authentication
- **Password**: Optional password for authentication
- **Timeout**: Query timeout in seconds (default: 30)

### Connection Testing

Use the "Save & Test" button to verify your connection settings.

## Query Types

### Custom AQL Queries

Write custom AQL queries with full flexibility. The plugin supports several built-in variables:

- `$__timeFrom` - Start of the selected time range (timestamp)
- `$__timeTo` - End of the selected time range (timestamp)
- `$__timeFilter(field)` - Expands to: `field >= $__timeFrom AND field <= $__timeTo`

#### Advanced Return Syntax

The plugin supports a convenient shorthand syntax for complex data flattening:

**Shorthand Syntax:**
```aql
FOR doc IN orders
  LIMIT 10
  RETURN {doc, doc.customer, doc.items}
```

This automatically converts to valid AQL:
```aql
FOR doc IN orders
  LIMIT 10
  RETURN {doc: doc, docCustomer: doc.customer, docItems: doc.items}
```

**Flattening Behavior:**
- `doc` → Returns all root-level fields as individual columns
- `doc.customer` → Flattens nested object as `customer.name`, `customer.email`, etc.
- `doc.items` → Flattens array elements as `items[0].product`, `items[0].quantity`, `items[1].product`, etc.

#### Example AQL Queries

**Basic time-filtered query:**
```aql
FOR doc IN my_collection
  FILTER $__timeFilter(doc.timestamp)
  SORT doc.timestamp ASC
  RETURN doc
```

**Complex data flattening:**
```aql
FOR doc IN orders
  FILTER doc.status == "completed"
  RETURN {doc, doc.customer, doc.customer.address, doc.items}
```

**Specific array element:**
```aql
FOR doc IN orders
  RETURN {doc, doc.items[0]}
```

**Aggregation query:**
```aql
FOR doc IN metrics
  FILTER $__timeFilter(doc.created_at)
  COLLECT timeGroup = DATE_ROUND(doc.created_at, 'PT1H') INTO g
  RETURN {
    time: timeGroup,
    value: AVG(g[*].doc.value)
  }
```

**Multi-collection query:**
```aql
FOR user IN users
  FOR event IN events
    FILTER event.user_id == user._key
    FILTER $__timeFilter(event.timestamp)
    RETURN {
      user: user.name,
      event: event.type,
      timestamp: event.timestamp
    }
```

### Collection-based Queries

Use the visual query builder for simpler queries with advanced field flattening:

1. **Collection**: Select the target collection
2. **Time Field**: Choose a field for time-based filtering
3. **Value Field**: Select the primary value field for metrics
4. **Filter**: Add custom filter conditions (AQL syntax) - automatically prefixes fields with `doc.`
5. **Group By**: Group results by a specific field
6. **Aggregation**: Apply COUNT, SUM, AVG, MIN, or MAX functions
7. **Sort**: Configure result ordering
8. **Limit**: Limit the number of returned documents
9. **Return Fields**: Specify which fields to include and flatten

#### Return Fields Advanced Usage

The **Return Fields** input supports sophisticated data flattening:

**Leave Empty**: Returns all root-level fields only (nested objects become JSON strings)

**Basic Fields**: 
- `product_name` → Returns just the product_name field
- `status, priority` → Returns multiple specific fields

**Root + Nested Flattening**:
- `doc` → All root-level fields as individual columns
- `doc, customer` → Root fields + flattened customer object
- `doc, customer, address` → Root + flattened customer + flattened address

**Array Flattening**:
- `items` → Flattens entire array as `items[0].product`, `items[1].product`
- `items[0]` → Flattens only the first array element
- `doc, customer, items` → Root + customer + all items array elements

**Complex Example**:
```
doc, customer, customer.address, items, tags
```
Results in columns like:
- `_key`, `order_id`, `status` (from doc)
- `customer.name`, `customer.email` (from customer)
- `customer.address.street`, `customer.address.city` (from address)
- `items[0].product`, `items[0].quantity` (from items array)
- `tags[0]`, `tags[1]` (from tags array)

## Data Structure Handling

### Nested Objects and Arrays

The plugin intelligently handles complex ArangoDB document structures:

#### Nested Objects
```json
{
  "_key": "order_001",
  "order_id": "ORD-2025-001",
  "status": "completed",
  "customer": {
    "name": "John Doe",
    "email": "john@example.com",
    "address": {
      "street": "123 Main St",
      "city": "New York",
      "country": "USA"
    }
  }
}
```

**Flattening Result:**
- `customer.name` → "John Doe"
- `customer.email` → "john@example.com"
- `customer.address.street` → "123 Main St"
- `customer.address.city` → "New York"
- `customer.address.country` → "USA"

#### Arrays
```json
{
  "_key": "order_001",
  "order_id": "ORD-2025-001",
  "items": [
    {
      "product": "Laptop",
      "quantity": 1,
      "price": 999.99
    },
    {
      "product": "Mouse",
      "quantity": 2,
      "price": 25.50
    }
  ],
  "tags": ["electronics", "business", "urgent"]
}
```

**Flattening Result:**
- `items[0].product` → "Laptop"
- `items[0].quantity` → 1
- `items[0].price` → 999.99
- `items[1].product` → "Mouse"
- `items[1].quantity` → 2
- `items[1].price` → 25.50
- `tags[0]` → "electronics"
- `tags[1]` → "business"
- `tags[2]` → "urgent"

#### Heterogeneous Schemas

The plugin handles documents with different field structures in the same collection:

```json
// Document 1
{
  "_key": "user_001", 
  "name": "John Doe", 
  "email": "john@example.com",
  "preferences": {
    "theme": "dark",
    "language": "en"
  }
}

// Document 2  
{
  "_key": "user_002", 
  "name": "Jane Smith", 
  "phone": "+1234567890",
  "preferences": {
    "notifications": true
  }
}
```

**Result:** Creates columns for all discovered fields, with empty values for missing fields:
- `name`: "John Doe", "Jane Smith"
- `email`: "john@example.com", ""
- `phone`: "", "+1234567890"
- `preferences.theme`: "dark", ""
- `preferences.language`: "en", ""
- `preferences.notifications`: "", true

## Template Variables

The datasource supports Grafana template variables for dynamic queries:

### Collection Variables
Create a variable with the query `collections` to list all available collections.

### Field Variables
Create a variable with the query `fields:collection_name` to list all fields in a specific collection.

## Time Series Data

For time series visualization, ensure your documents have:
- A timestamp field (can be Unix timestamp, ISO string, or ArangoDB datetime)
- A numeric value field for metrics
- Optional grouping fields for multi-series charts

Example document structure:
```json
{
  "_key": "metric_001",
  "timestamp": 1642781234000,
  "value": 42.5,
  "sensor_id": "temp_01",
  "location": "room_a"
}
```

## Performance Tips

1. **Indexes**: Create appropriate indexes on fields used in filters and time ranges
2. **Limits**: Use reasonable limits to avoid loading too much data
3. **Time Filters**: Always use time-based filtering for large collections
4. **Aggregation**: Use ArangoDB's built-in aggregation functions rather than loading raw data
5. **Field Selection**: Use Return Fields to limit data and control flattening scope
6. **Array Targeting**: For large arrays, consider targeting specific elements (e.g., `items[0]`) instead of flattening entire arrays
7. **Batch Size**: Configure appropriate batch sizes in datasource settings for large result sets

## Example Indexes

```aql
// Time-based index
db.my_collection.ensureIndex({
  type: "persistent",
  fields: ["timestamp"]
});

// Compound index for filtered time queries
db.my_collection.ensureIndex({
  type: "persistent", 
  fields: ["status", "timestamp"]
});
```

## Development

### Building the Plugin

```bash
# Install dependencies
npm install

# Build for development
npm run dev

# Build for production
npm run build

# Run tests
npm run test

# Watch mode for development
npm run watch
```

### Project Structure

```
src/
├── module.ts          # Plugin entry point
├── datasource.ts      # Main datasource implementation  
├── types.ts           # TypeScript type definitions
├── ConfigEditor.tsx   # Datasource configuration UI
└── QueryEditor.tsx    # Query editor UI
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

Apache License 2.0

## Support

- Create issues on GitHub for bug reports
- Check ArangoDB documentation for AQL syntax: https://www.arangodb.com/docs/stable/aql/
- Grafana plugin development docs: https://grafana.com/docs/grafana/latest/developers/plugins/

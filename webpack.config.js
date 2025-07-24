const path = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin');

module.exports = {
  entry: './src/module.ts',
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: 'module.js',
    libraryTarget: 'system',
  },
  externals: [
    '@grafana/data',
    '@grafana/runtime',
    '@grafana/ui',
    'react',
    'react-dom',
    'rxjs',
    'rxjs/operators',
  ],
  resolve: {
    extensions: ['.ts', '.tsx', '.js'],
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
    ],
  },
  plugins: [
    new CopyWebpackPlugin({
      patterns: [
        { from: 'plugin.json', to: '.' },
        { from: 'README.md', to: '.' },
        { from: 'img', to: 'img', noErrorOnMissing: true },
      ],
    }),
  ],
  optimization: {
    minimize: false,
  },
};

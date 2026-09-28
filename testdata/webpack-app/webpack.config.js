module.exports = {
  mode: 'production',
  entry: './src/main.js',
  output: {
    filename: '[name].js',
    chunkFilename: '[name].js',
    path: __dirname + '/dist',
    clean: true,
  },
  devtool: false,
  stats: {
    all: false,
    chunks: true,
    chunkModules: true,
    nestedModules: true,
    dependentModules: true,
    orphanModules: true,
    modules: true,
    assets: false,
    errors: true,
    warnings: true,
  },
  optimization: {
    minimize: true,
  },
}

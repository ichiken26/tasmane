// cf の Wrangler bundler 用設定（静的アセットの出力先）
// CLI 操作は cf を使う。wrangler パッケージは Build Output 生成の bundler として内部利用するのみ
import { defineWranglerConfig } from 'wrangler/experimental-config'

export default defineWranglerConfig({
  assetsDirectory: '.output/public',
})

// Nuxt Worker（tasmane-web）の cf 設定
// Go API は自宅サーバーで動かし、Cloudflare Tunnel 上の Workers VPC Service 経由でのみ到達する
import { bindings, defineConfig } from 'cf/config'

// Tunnel 先 Go API の VPC Service ID（`cf workers-vpc services create` の結果）
const apiVpcServiceId = process.env.TASMANE_API_VPC_SERVICE_ID
if (!apiVpcServiceId || !/^[0-9a-f-]{36}$/u.test(apiVpcServiceId))
  throw new Error('TASMANE_API_VPC_SERVICE_ID (Workers VPC Service ID of the home-server Go API) is required')

export default defineConfig({
  worker: {
    name: 'tasmane-web',
    // Nitro cloudflare_module 出力
    entrypoint: '.output/server/index.mjs',
    compatibilityDate: '2025-05-15',
    compatibilityFlags: ['nodejs_compat'],
    // 公開ホスト名（唯一の公開入口）
    domains: [process.env.TASMANE_PUBLIC_HOSTNAME || 'tasmane.kokage-studio.com'],
    // workers.dev / Preview URL は公開しない（入口はカスタムドメインのみ）
    workersDev: false,
    previewUrls: false,
    observability: { enabled: true, headSamplingRate: 1 },
    env: {
      // 自宅サーバー Go API（Tunnel 経由・非公開）
      TASMANE_API: bindings.vpcService({ id: apiVpcServiceId, dev: { remote: true } }),
      ASSETS: bindings.assets(),
      // Worker Secrets（値は `cf workers secrets bulk` で投入し Git に置かない）
      NUXT_GOOGLE_CLIENT_ID: bindings.secret(),
      NUXT_GOOGLE_CLIENT_SECRET: bindings.secret(),
      NUXT_GOOGLE_REDIRECT_URI: bindings.secret(),
      NUXT_SESSION_SECRET: bindings.secret(),
      NUXT_INTERNAL_BINDING_SECRET: bindings.secret(),
      // ログイン許可メールアドレス（カンマ区切り）
      NUXT_ALLOWED_EMAILS: bindings.secret(),
    },
  },
})

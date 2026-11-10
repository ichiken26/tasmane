// 401 時にログインへ転送してよいかの判定
// Chrome の先読み（prerender）中のページが転送すると OAuth のトランザクション Cookie を上書きし、
// 実際に表示中のログインが state 不一致や重複コールバックで失敗するため、表示中のページからだけ転送する
export function shouldRedirectToLogin(page: { prerendering: boolean; pathname: string }): boolean {
  return !page.prerendering && !page.pathname.startsWith('/auth/')
}

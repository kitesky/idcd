export default {
  logo: <span>idcd 文档</span>,
  project: {
    link: 'https://github.com/kite365/idcd'
  },
  docsRepositoryBase: 'https://github.com/kite365/idcd/tree/main/apps/docs',
  footer: {
    text: '© 2026 idcd'
  },
  useNextSeoProps() {
    return {
      titleTemplate: '%s – idcd 文档'
    }
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta property="og:title" content="idcd 文档" />
      <meta property="og:description" content="全球网络诊断工具文档" />
    </>
  )
}

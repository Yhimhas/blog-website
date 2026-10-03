import { createHash } from 'node:crypto'
import { copyFile, mkdir, readFile, stat, writeFile, chmod } from 'node:fs/promises'
import { constants } from 'node:fs'

const [version, ytdlp] = process.argv.slice(2)
if (!/^\d+\.\d+\.\d+$/.test(version ?? '') || !ytdlp) {
  throw new Error('Usage from release root: node deploy/prepare-tooling.mjs <pnpm-version> <existing-yt-dlp>')
}
const [major, minor] = version.split('.').map(Number)
if (major < 11 || (major === 11 && minor < 19)) {
  throw new Error('pnpm must satisfy >=11.19.0')
}
const packageJson = JSON.parse(await readFile('frontend/package.json', 'utf8'))
if (packageJson.engines?.pnpm !== '>=11.19.0') throw new Error('Review changed pnpm engine policy before preparing tooling')
if (!(await stat(ytdlp)).isFile()) throw new Error('yt-dlp must be an existing regular executable file')
const metadataResponse = await fetch(`https://registry.npmjs.org/pnpm/${version}`, { signal: AbortSignal.timeout(30_000) })
if (!metadataResponse.ok) throw new Error(`pnpm metadata HTTP ${metadataResponse.status}`)
const metadata = await metadataResponse.json()
const source = new URL(metadata.dist.tarball)
if (source.protocol !== 'https:' || source.hostname !== 'registry.npmjs.org') throw new Error('Unexpected archive origin')
const archiveResponse = await fetch(source, { signal: AbortSignal.timeout(60_000) })
if (!archiveResponse.ok) throw new Error(`pnpm archive HTTP ${archiveResponse.status}`)
const archive = Buffer.from(await archiveResponse.arrayBuffer())
const integrity = `sha512-${createHash('sha512').update(archive).digest('base64')}`
if (integrity !== metadata.dist.integrity) throw new Error('pnpm archive integrity mismatch')
await mkdir('tooling', { recursive: true })
await writeFile('tooling/pnpm.tgz', archive, { flag: 'wx' })
await copyFile(ytdlp, 'tooling/yt-dlp', constants.COPYFILE_EXCL)
await chmod('tooling/yt-dlp', 0o755)
await writeFile('tooling/versions.json', JSON.stringify({ pnpm: version, integrity }, null, 2), { flag: 'wx' })
console.log(`Verified pnpm ${version} archive; tooling retained`)

import test from 'node:test'
import assert from 'node:assert/strict'
import { effectScope } from 'vue'
import { useMusicPlayer } from '../src/useMusicPlayer.ts'
import { fetchMusicLibrary } from '../src/musicLibraryApi.ts'
import { parseCapability, PlaybackError } from '../src/musicPlaybackApi.ts'
class AudioMock extends EventTarget {
 static instances=[]; paused=true; currentTime=0; duration=80; volume=1; ended=false; src=''
 constructor(){super();AudioMock.instances.push(this)}
 async play(){if(AudioMock.reject){throw new DOMException('denied','NotAllowedError')};this.paused=false;this.dispatchEvent(new Event('playing'))}
 pause(){this.paused=true;this.dispatchEvent(new Event('pause'))}
 load(){} removeAttribute(){this.src=''}
}
const track={id:'bilibili:BV1a4MS67Eey:1',title:'one',artist:'author',platform:'bilibili',playlistId:'p',url:'https://www.bilibili.com/video/BV1a4MS67Eey/',availability:'unknown'}
const session=id=>({sessionId:id,status:'ready',streamUrl:`/api/v1/music/streams/${id}`,durationSeconds:80,seekMode:'none',expiresAt:new Date(Date.now()+60000).toISOString()})
const response=data=>new Response(JSON.stringify({data}),{headers:{'Content-Type':'application/json'}})
test('late creation cannot replace latest audio; stop releases source',async()=>{
 globalThis.Audio=AudioMock;let resolveFirst;let calls=0;const stops=[]
 globalThis.fetch=async(url,options)=>{if(url.endsWith('/stop')){stops.push(url);return new Response(null,{status:204})};if(options.method==='POST'){calls++;if(calls===1)return new Promise(r=>resolveFirst=r);return response(session('second'))};return response(session('second'))}
 const scope=effectScope();const p=scope.run(()=>useMusicPlayer());const first=p.play(track,[track]);await new Promise(r=>setImmediate(r));await p.play({...track,id:'two'},[track]);resolveFirst(response(session('first')));await first
 assert.equal(p.track.value.id,'two');assert.equal(p.state.value,'playing');assert.ok(stops.some(x=>x.includes('first')));const audio=AudioMock.instances.at(-1);p.stop();assert.equal(audio.src,'');scope.stop()
})
test('autoplay rejection is visible; media error does not skip songs',async()=>{
 globalThis.Audio=AudioMock;AudioMock.reject=true;globalThis.fetch=async(url,options)=>url.endsWith('/stop')?new Response(null,{status:204}):response(session('id'))
 const scope=effectScope();const p=scope.run(()=>useMusicPlayer());await p.play(track,[track,{...track,id:'next'}]);assert.equal(p.state.value,'blocked');AudioMock.reject=false;await p.toggle();assert.equal(p.state.value,'playing');AudioMock.instances.at(-1).dispatchEvent(new Event('error'));assert.equal(p.track.value.id,track.id);assert.equal(p.state.value,'error');scope.stop()
})
test('library reads remaining pages instead of searching first page only',async()=>{
 let pages=0;const raw={id:track.id,title:'one',provider:'bilibili',sourceUrl:track.url,availability:'unknown'}
 globalThis.fetch=async url=>{if(url.endsWith('/playlists'))return response([{id:'p',provider:'bilibili',title:'p',sourceUrl:'https://space.bilibili.com/1/favlist?fid=1',syncStatus:'ready'}]);pages++;return new Response(JSON.stringify({data:[{...raw,id:`t${pages}`}],pagination:{total:2}}))}
 const data=await fetchMusicLibrary(new AbortController().signal);assert.equal(pages,2);assert.equal(data[0].tracks.length,2)
})

import { migrateFavoriteID, readFavoriteSnapshots, missingFavorite } from '../src/musicFavorites.ts'
test('legacy favorite migration keeps known P1 and unknown favorites',()=>{
 assert.equal(migrateFavoriteID('bilibili:BV1a4MS67Eey'), 'bilibili:BV1a4MS67Eey:1')
 assert.equal(migrateFavoriteID('legacy-unknown'), 'legacy-unknown')
 assert.equal(missingFavorite('legacy-unknown').availability, 'unavailable')
 const values=readFavoriteSnapshots({data:{items:[{id:'bilibili:BV1a4MS67Eey'}, {id:'legacy-unknown'}]}})
 assert.equal(values.length,2);assert.equal(values[0].partId,'1');assert.equal(values[1].id,'legacy-unknown')
})
test('expired pause requires restart and route disposal stops the session',async()=>{
 globalThis.Audio=AudioMock;AudioMock.reject=false;let expired=false,stopped=false
 globalThis.fetch=async(url,options)=>{if(url.endsWith('/stop')){stopped=true;return new Response(null,{status:204})};if(expired)return new Response(JSON.stringify({error:{code:'SESSION_EXPIRED'}}),{status:410});return response(session('pause'))}
 const scope=effectScope();const p=scope.run(()=>useMusicPlayer());await p.play(track,[track]);await p.toggle();assert.equal(p.state.value,'paused');expired=true;await p.toggle();assert.equal(p.state.value,'error');assert.match(p.message.value,/重新播放/);scope.stop();assert.equal(stopped,true)
})

test('NetEase uses unified playback and preserves official fallback on refusal', async()=>{
 globalThis.Audio=AudioMock;AudioMock.reject=false;let requested='',deny=false
 globalThis.fetch=async(url,options)=>{if(url.endsWith('/stop'))return new Response(null,{status:204});if(options.method==='POST')requested=JSON.parse(options.body).trackId;if(deny)return new Response(JSON.stringify({error:{code:'AUDIO_SOURCE_UNAVAILABLE'}}),{status:422});return response(session('netease'))}
 const scope=effectScope();const p=scope.run(()=>useMusicPlayer());const netease={...track,id:'netease:123',platform:'netease',url:'https://music.163.com/#/song?id=123'}
 await p.play(netease,[netease]);assert.equal(requested,'netease:123');assert.equal(p.state.value,'playing');deny=true;await p.play(netease,[netease]);assert.equal(p.state.value,'error');assert.match(p.message.value,/官方音源/);assert.equal(p.track.value.url,netease.url);scope.stop()
})

test('preview ends explicitly, releases its session and never auto-advances', async()=>{
 globalThis.Audio=AudioMock;AudioMock.reject=false;let creates=0,stops=0
 const capability={mediaKind:'preview',trackDurationSeconds:200,streamDurationSeconds:30,previewStartSeconds:30,previewEndSeconds:60}
 globalThis.fetch=async(url,options)=>{if(url.endsWith('/stop')){stops++;return new Response(null,{status:204})};if(options.method==='POST')creates++;return response({...session('preview'),capability})}
 const scope=effectScope();const p=scope.run(()=>useMusicPlayer());await p.play(track,[track,{...track,id:'next'}]);assert.equal(p.capability.value.mediaKind,'preview');assert.equal(p.duration.value,30)
 AudioMock.instances.at(-1).dispatchEvent(new Event('ended'));await new Promise(r=>setImmediate(r));assert.equal(p.state.value,'preview-ended');assert.match(p.message.value,/试听结束/);assert.equal(creates,1);assert.equal(stops,1);scope.stop()
})

test('missing completeness evidence remains unknown even after browser metadata',async()=>{
 globalThis.Audio=AudioMock;AudioMock.reject=false;globalThis.fetch=async(url)=>url.endsWith('/stop')?new Response(null,{status:204}):response(session('unknown'))
 const scope=effectScope();const p=scope.run(()=>useMusicPlayer());await p.play(track,[track]);const a=AudioMock.instances.at(-1);a.dispatchEvent(new Event('loadedmetadata'));assert.equal(p.capability.value.mediaKind,'unknown');a.dispatchEvent(new Event('ended'));assert.match(p.message.value,/完整性未确认/);scope.stop()
})

test('capability and machine-readable failures are validated independently',()=>{
 assert.equal(parseCapability(undefined).mediaKind,'unknown')
 const valid={mediaKind:'full',trackDurationSeconds:200,streamDurationSeconds:200,previewStartSeconds:null,previewEndSeconds:null}
 assert.equal(parseCapability(valid).mediaKind,'full')
 for(const raw of [null,{...valid,mediaKind:'fake'},{...valid,streamDurationSeconds:-1},{...valid,mediaKind:'preview'},{...valid,mediaKind:'preview',previewStartSeconds:60,previewEndSeconds:30}])assert.throws(()=>parseCapability(raw))
 assert.equal(new PlaybackError('ENTITLEMENT_REQUIRED').code,'ENTITLEMENT_REQUIRED')
})

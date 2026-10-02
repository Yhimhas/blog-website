import test from 'node:test'
import assert from 'node:assert/strict'
import { effectScope, createRenderer, h, nextTick } from 'vue'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import { useMusicPlayer, provideMusicPlayer } from '../src/useMusicPlayer.ts'
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
 const scope=effectScope();const p=scope.run(()=>useMusicPlayer());const first=p.play(track,[track]);await new Promise(r=>setImmediate(r));const second=p.play({...track,id:'two'},[track]);resolveFirst(response(session('first')));await Promise.all([first,second])
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
test('expired pause requires restart and owner disposal stops the session',async()=>{
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

test('seeking recreates the stream at an offset, preserves pause and volume, and can seek backwards', async()=>{
 globalThis.Audio=AudioMock;AudioMock.reject=false;const requests=[],stops=[]
 const capability={mediaKind:'preview',trackDurationSeconds:200,streamDurationSeconds:30,previewStartSeconds:60,previewEndSeconds:90}
 globalThis.fetch=async(url,options)=>{
  if(url.endsWith('/stop')){stops.push(url);return new Response(null,{status:204})}
  if(options.method==='POST'){const body=JSON.parse(options.body);requests.push(body);return response({...session(`seek-${requests.length}`),seekMode:'restart',startSeconds:body.startSeconds,capability})}
  return response(session(`seek-${requests.length}`))
 }
 const scope=effectScope(),p=scope.run(()=>useMusicPlayer());p.setVolume(.35)
 await p.play(track,[track]);await p.seek(20)
 assert.equal(requests.at(-1).startSeconds,20);assert.equal(p.currentTime.value,20);assert.equal(p.duration.value,30);assert.equal(p.state.value,'playing');assert.equal(stops.length,1)
 const a=AudioMock.instances.at(-1);a.currentTime=2;a.dispatchEvent(new Event('timeupdate'));assert.equal(p.currentTime.value,22);assert.equal(a.volume,.35)
 await p.toggle();await p.seek(5);assert.equal(p.state.value,'paused');assert.equal(AudioMock.instances.at(-1).paused,true);assert.equal(p.currentTime.value,5)
 await p.seek(999);assert.equal(requests.at(-1).startSeconds,29.9);assert.equal(p.capability.value.mediaKind,'preview')
 const count=requests.length;await p.seek(NaN);assert.equal(requests.length,count);scope.stop()
})

test('stop during creation releases the late session without starting audio',async()=>{
 globalThis.Audio=AudioMock;let complete;const stops=[];const count=AudioMock.instances.length
 globalThis.fetch=async(url)=>{if(url.endsWith('/stop')){stops.push(url);return new Response(null,{status:204})};return new Promise(r=>complete=r)}
 const scope=effectScope(),p=scope.run(()=>useMusicPlayer());const pending=p.play(track,[track]);await new Promise(r=>setImmediate(r));p.stop();complete(response(session('late')));await pending
 assert.equal(p.state.value,'stopped');assert.equal(AudioMock.instances.length,count);assert.equal(stops.length,1);scope.stop()
})

test('actual RouterView unmounts music but retains the app-owned audio and queue',async()=>{
 globalThis.Audio=AudioMock;AudioMock.reject=false;let stops=0;const seen=[]
 globalThis.fetch=async(url)=>{if(url.endsWith('/stop')){stops++;return new Response(null,{status:204})};return response(session('shared'))}
 const node=()=>({children:[],parent:null})
 const renderer=createRenderer({
  createElement:node,createText:node,createComment:node,setText(){},setElementText(){},patchProp(){},
  insert(child,parent,anchor){if(child.parent){const i=child.parent.children.indexOf(child);if(i>=0)child.parent.children.splice(i,1)};child.parent=parent;const i=parent.children.indexOf(anchor);parent.children.splice(i<0?parent.children.length:i,0,child)},
  remove(child){const i=child.parent?.children.indexOf(child);if(i>=0)child.parent.children.splice(i,1);child.parent=null},
  parentNode:n=>n.parent,nextSibling:n=>n.parent?.children[n.parent.children.indexOf(n)+1]||null
 })
 const Music={setup(){seen.push(useMusicPlayer());return()=>h('div','music')}}
 const Other={setup(){seen.push(useMusicPlayer());return()=>h('div','blog')}}
 const router=createRouter({history:createMemoryHistory(),routes:[{path:'/music',component:Music},{path:'/blog',component:Other}]})
 let player;const Root={setup(){player=provideMusicPlayer();return()=>h(RouterView)}}
 await router.push('/music');const app=renderer.createApp(Root).use(router);app.mount(node());await nextTick()
 await player.play(track,[track]);const audio=AudioMock.instances.at(-1)
 await router.push('/blog');await nextTick();assert.equal(stops,0);assert.equal(audio.paused,false);assert.equal(seen.at(-1),player)
 await player.toggle();assert.equal(audio.paused,true)
 await router.push('/music');await nextTick();assert.equal(seen.at(-1),player);assert.equal(player.queue.value[0].id,track.id);await player.toggle();assert.equal(AudioMock.instances.at(-1),audio);assert.equal(audio.paused,false)
 app.unmount();assert.equal(audio.paused,true);assert.equal(stops,1)
})

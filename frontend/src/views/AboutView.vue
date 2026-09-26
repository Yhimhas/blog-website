<script setup lang="ts">
import { MotionReveal, OrbitField } from '@field-lab/vue'
import { RouterLink } from 'vue-router'
import { posts, categories } from '../content'
</script>

<template>
  <div class="about-page">
    <div class="about-coordinate"><span>03 / BEHIND THE SCREEN</span><span>PERSONAL FILE — 001</span></div>
    <section class="profile-hero">
      <MotionReveal class="profile-copy">
        <div class="profile-id"><img src="/yhimhas-logo.jpg" alt="Yhimhas 的头像" width="56" height="56"><span>YHIMHAS / Lin<br><small>记录者 · 构建者 · 音乐爱好者</small></span></div>
        <h1>保持好奇。<br>让想法<span>慢慢成形。</span></h1>
        <p>你好，我是 Lin，一个正在探索前端开发的人。<br>这里是我的数字自留地，也是一个持续生长的个人实验场。</p>
        <RouterLink class="primary-link" to="/blog">读一读我的记录 <span>↗</span></RouterLink>
      </MotionReveal>
      <div class="profile-visual">
        <div class="profile-visual-label"><span>IDEAS IN ORBIT</span><span>● EXPLORING</span></div>
        <OrbitField variant="orbit" :density="1400" color="#bfc9b5" accent="#d4ef37" class="profile-orbit" />
        <div class="profile-visual-footer"><strong>未完成，<br>也在前进。</strong><span>CODE / SOUND / LIFE<br>EST. 2026</span></div>
      </div>
    </section>
    <div class="profile-facts"><div><strong>{{ String(posts.length).padStart(2, '0') }}</strong><span>篇文字记录</span></div><div><strong>{{ String(categories.length).padStart(2, '0') }}</strong><span>个探索方向</span></div><div><strong>∞</strong><span>对日常的好奇</span></div></div>
    <MotionReveal><section class="profile-statement"><div class="eyebrow">01 / A LITTLE ABOUT ME</div><div><h2>把学到的留下，<br>把喜欢的分享。</h2><p>写代码时，我喜欢从一个小组件开始，把模糊的想法一点点变成可以触摸的界面。离开编辑器，音乐是另一种整理思绪的方式。</p><p>这个小站存放开发笔记、喜欢的旋律，以及不想忘记的生活切片。不追求一次完成所有事情，只希望每一次更新，都更接近自己的想法。</p><div class="profile-tags"><span>Vue 3</span><span>TypeScript</span><span>Music lover</span><span>Lifelong learner</span></div></div></section></MotionReveal>
    <MotionReveal :delay="80"><section class="profile-interests"><div class="eyebrow">02 / THINGS THAT MATTER</div><div class="interest-grid"><article><span>01 / BUILD</span><h3>构建小而具体的东西</h3><p>从组件、交互到完整页面，在动手的过程中找到答案。</p><RouterLink to="/blog">探索开发笔记 ↗</RouterLink></article><article><span>02 / LISTEN</span><h3>为生活留一条音轨</h3><p>专注时的环境音乐，夜间的旋律，让声音陪伴日常。</p><RouterLink to="/music">进入音乐空间 ↗</RouterLink></article><article><span>03 / RECORD</span><h3>认真对待普通的一天</h3><p>一些观察，一点反思。记录让平常的瞬间有迹可循。</p><RouterLink to="/blog">翻阅文字档案 ↗</RouterLink></article></div></section></MotionReveal>
    <div class="profile-end"><span>THANK YOU FOR STOPPING BY.</span><p>很高兴，在这里遇见你。</p><RouterLink to="/">返回首页 ↗</RouterLink></div>
  </div>
</template>

<style scoped>
.about-page{padding:30px 4.4% 0}.about-coordinate{display:flex;justify-content:space-between;font:10px var(--f-detail);letter-spacing:.1em;color:#747d69}.profile-hero{display:grid;grid-template-columns:1.1fr 1fr;gap:48px;margin:36px 0}.profile-copy{padding:20px 0}.profile-id{display:flex;gap:16px;align-items:center;font:12px/1.8 var(--f-mono)}.profile-id img{border-radius:50%;filter:grayscale(1)}.profile-id small{font:11px var(--f-detail);color:#737b69}.profile-copy h1{font-size:clamp(36px,4.6vw,68px);line-height:1.28;margin:32px 0;letter-spacing:-.06em}.profile-copy h1 span{color:#6c7a40}.profile-copy p{font-size:14px;line-height:2;color:#6d7564}.profile-copy .primary-link{max-width:240px;margin-top:30px;font-size:13px}.profile-visual{position:relative;min-height:450px;background:radial-gradient(ellipse,#33412d,#171e18 75%);color:#e8eddf;overflow:hidden}.profile-visual-label,.profile-visual-footer{position:absolute;left:25px;right:25px;display:flex;justify-content:space-between;z-index:1}.profile-visual-label{top:24px;font:9px var(--f-detail);color:#c2cca9}.profile-orbit{height:450px}.profile-visual-footer{bottom:25px;align-items:end}.profile-visual-footer strong{font-size:26px;font-weight:500;line-height:1.6}.profile-visual-footer span{font:9px/2 var(--f-detail);color:#9ca98d;text-align:right}.profile-facts{display:grid;grid-template-columns:repeat(3,1fr);border-block:1px solid #cfd3c6}.profile-facts>div{padding:25px;display:flex;align-items:center;gap:20px;border-right:1px solid #cfd3c6}.profile-facts>div:last-child{border:0}.profile-facts strong{font:36px var(--f-mono)}.profile-facts span{font-size:12px;color:#77816a}.profile-statement{padding:64px 0;display:grid;grid-template-columns:1fr 2fr;gap:32px}.profile-statement h2{font-size:32px;line-height:1.5;margin:0 0 24px}.profile-statement p{max-width:640px;font-size:14px;line-height:2;color:#6c7562}.profile-tags{display:flex;flex-wrap:wrap;gap:8px;margin-top:26px}.profile-tags span{font-size:11px;border:1px solid #c5ccb9;padding:7px 12px}.profile-interests{border-top:1px solid #cfd3c6;padding:32px 0 50px}.interest-grid{display:grid;grid-template-columns:repeat(3,1fr);margin-top:25px;gap:20px}.interest-grid article{padding:26px;background:#e5e8dd;border-top:2px solid #727f58}.interest-grid article>span{font:10px var(--f-detail);color:#6d785d}.interest-grid h3{font-size:18px;margin:30px 0 15px}.interest-grid p{font-size:13px;line-height:1.9;color:#6e7862;min-height:50px}.interest-grid a{display:inline-block;font-size:12px;margin-top:18px;border-bottom:1px solid #a4af8d;padding-bottom:6px}.profile-end{padding:30px 0;border-top:1px solid #cfd3c6;display:flex;align-items:center;justify-content:space-between;gap:20px}.profile-end span,.profile-end a{font-size:10px;color:#6e7862}.profile-end p{font-size:18px}@media(max-width:900px){.profile-hero{gap:24px}.interest-grid{gap:12px}.interest-grid article{padding:18px}}@media(max-width:760px){.profile-hero,.profile-statement{grid-template-columns:1fr}.profile-hero{margin-top:20px}.profile-visual{min-height:340px}.profile-orbit{height:340px}.about-coordinate span:last-child{display:none}.profile-facts>div{padding:18px 8px;flex-direction:column;gap:8px}.profile-facts strong{font-size:28px}.profile-facts span{font-size:11px}.profile-statement{padding:40px 0;gap:20px}.interest-grid{grid-template-columns:1fr}.interest-grid h3{margin-top:18px}.interest-grid p{min-height:0}.profile-end{align-items:start;flex-direction:column;gap:8px}}
</style>

<script setup lang="ts">
import SiteIcon from "../components/SiteIcon.vue";
import { RouterLink } from "vue-router"
</script>
<template>
      <section id="home" class="hero">
        <div class="hero-copy">
          <div class="eyebrow">
            <SiteIcon class="small-cross" name="sparkle" /> A PERSONAL SPACE ON THE INTERNET
          </div>
          <h1>
            在代码与旋律之间，<br />记录<span class="outlined">正在发生</span
            >的生活<span class="accent-square">.</span>
          </h1>
          <div class="hero-actions">
            <RouterLink class="primary-link" to="/blog"
              >探索我的文字 <SiteIcon name="arrow" /></RouterLink
            ><RouterLink class="quiet-link" to="/music">听见日常 <SiteIcon name="arrow" /></RouterLink>
          </div>
          <div class="hero-footnote">
            <span>独立记录 / 持续生长</span><span>EST. 2026 <i>↘</i></span>
          </div>
        </div>
        <!-- <FieldProvider theme="dark" :paused="paused" class="hero-visual">
          <div class="visual-top"><span>FIELD / 001</span><span><b class="status-dot" /> EXPLORING</span></div>
          <div class="orbital-label">THOUGHTS IN ORBIT</div>
          <OrbitField variant="core" :density="1400" color="#bfc9b5" accent="#d4ef37" class="hero-orbit" />
          <div class="visual-caption"><span>让好奇心<br><strong>保持运行。</strong></span><span class="coordinate">PERSONAL / ARCHIVE<br>IDEAS · CODE · SOUND</span></div>
          <span class="visual-cross">+</span>
        </FieldProvider> -->
      </section>
      <div class="index-strip">
        <span><b>01—04</b> PERSONAL ARCHIVE</span
        ><span>BUILD THINGS. COLLECT MOMENTS.</span
        ><RouterLink to="/blog">进入博客 <SiteIcon name="arrow" /></RouterLink>
      </div>
      <!-- <section id="about" class="about-section">
        <div>
          <div class="eyebrow">03 / BEHIND THE SCREEN</div>
          <h2>保持好奇。<br />慢慢构建，认真生活。</h2>
        </div>
        <div class="about-copy">
          <p>
            我是
            Lin，一个正在探索前端开发的人。这个小站是我的数字自留地：存放学到的知识、喜欢的音乐，以及不想忘记的瞬间。
          </p>
          <p>
            不追求一次完成所有事情，只希望每一次更新，都比上一次更接近自己的想法。
          </p>
          <div class="about-tags">
            <span>Vue 3</span><span>TypeScript</span><span>Music lover</span
            ><span>Lifelong learner</span>
          </div>
        </div>
      </section> -->
</template>

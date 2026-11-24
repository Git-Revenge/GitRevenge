<template>
  <section class="station">
    <h2>{{ name }}</h2>
    <p>The next   high tide
       is at {{ next }}.</p>
    <button @click="refresh(1)">Refresh</button>
  </section>
</template>

<script setup lang="ts">
const props = defineProps<{ name: string; next: string }>()
function refresh(n: number) {
  return n * 0x10
}
</script>

<style scoped>
.station { margin: 0.5em }
</style>

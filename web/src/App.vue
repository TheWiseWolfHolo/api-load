<script setup lang="ts">
import GlobalProviders from "@/components/GlobalProviders.vue";
import Layout from "@/components/Layout.vue";
import { useAuthKey } from "@/services/auth";
import { computed } from "vue";

const authKey = useAuthKey();
const isLoggedIn = computed(() => !!authKey.value);
</script>

<template>
  <global-providers>
    <layout v-if="isLoggedIn" />
    <router-view v-else />
  </global-providers>
</template>

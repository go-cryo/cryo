<template>
  <q-layout view="lHh Lpr lFf">
    <q-header bordered class="bg-white text-dark">
      <q-toolbar>
        <q-space />
      </q-toolbar>
    </q-header>

    <q-drawer
      :model-value="true"
      :breakpoint="0"
      bordered
      :width="280"
      class="cryo-drawer column no-wrap"
    >
      <!-- Logo section - centered -->
      <div class="q-pa-lg">
        <div class="column items-center q-gutter-sm">
          <img src="/icon.png" alt="Cryo" style="width: 80px; height: 80px;" />
          <span class="text-h5 text-weight-medium text-dark">Cryo</span>
          <span v-if="displayVersion" class="text-caption text-grey-6">{{ displayVersion }}</span>
        </div>
      </div>

      <q-separator class="q-mb-sm" />

      <!-- Navigation -->
      <q-list padding class="col">
        <q-item
          clickable
          v-ripple
          :to="{ name: 'home' }"
          exact
        >
          <q-item-section avatar>
            <q-icon name="home" />
          </q-item-section>
          <q-item-section>
            <q-item-label>Home</q-item-label>
            <q-item-label caption>Overview</q-item-label>
          </q-item-section>
        </q-item>

        <q-item
          clickable
          v-ripple
          :to="{ name: 'hosts' }"
        >
          <q-item-section avatar>
            <q-icon name="dns" />
          </q-item-section>
          <q-item-section>
            <q-item-label>Hosts</q-item-label>
            <q-item-label caption>Storage backends</q-item-label>
          </q-item-section>
        </q-item>

        <q-item
          clickable
          v-ripple
          :to="{ name: 'repositories' }"
        >
          <q-item-section avatar>
            <q-icon name="inventory_2" />
          </q-item-section>
          <q-item-section>
            <q-item-label>Repositories</q-item-label>
            <q-item-label caption>Restic backup targets</q-item-label>
          </q-item-section>
        </q-item>

        <q-item
          clickable
          v-ripple
          :to="{ name: 'backupjobs' }"
        >
          <q-item-section avatar>
            <q-icon name="schedule" />
          </q-item-section>
          <q-item-section>
            <q-item-label>Backup Jobs</q-item-label>
            <q-item-label caption>Scheduled backups</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>

      <q-space />

      <q-separator class="q-my-sm" />

      <!-- Settings section -->
      <q-list padding>
        <q-item clickable v-ripple :to="{ name: 'settings' }">
          <q-item-section avatar>
            <q-icon name="settings" />
          </q-item-section>
          <q-item-section>
            <q-item-label>Settings</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>
      <q-separator />
      <q-item class="q-py-md">
        <q-item-section avatar>
          <q-avatar size="40px" color="primary" text-color="white">
            <q-icon name="person" />
          </q-avatar>
        </q-item-section>
        <q-item-section>
          <q-item-label class="text-weight-medium">{{ currentUser || 'User' }}</q-item-label>
        </q-item-section>
        <q-item-section side v-if="authEnabled">
          <q-btn flat round dense icon="logout" color="grey" @click="onLogout" />
        </q-item-section>
        <q-item-section side v-else>
          <q-icon name="more_vert" color="grey" />
        </q-item-section>
      </q-item>
    </q-drawer>

    <q-page-container>
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import versionApi from 'src/api/version';
import authApi from 'src/api/auth';

const version = ref('');
const currentUser = ref('');
const authEnabled = ref(false);

const displayVersion = computed(() => {
  if (!version.value) return '';
  if (/^\d/.test(version.value)) return `v${version.value}`;
  return version.value;
});

onMounted(async () => {
  try {
    const response = await versionApi.getVersion();
    version.value = response.version;
  } catch (e) {
    console.error('Failed to fetch version:', e);
  }

  try {
    const info = await authApi.getAuthInfo();
    authEnabled.value = info.basicEnabled || info.oidcEnabled;
    if (authEnabled.value) {
      const user = await authApi.me();
      currentUser.value = user.username;
    }
  } catch {
    // auth not configured or not logged in
  }
});

async function onLogout() {
  try {
    await authApi.logout();
  } catch {
    // ignore
  }
  window.location.href = '/login';
}
</script>

<style lang="scss" scoped>
.cryo-drawer {
  background: #fafafa;

  :deep(.q-item.q-router-link--active) {
    background: rgba(2, 136, 209, 0.08);

    .q-item__label {
      font-weight: 500;
      color: #0288D1;
    }

    .q-icon {
      color: #0288D1;
    }
  }
}
</style>

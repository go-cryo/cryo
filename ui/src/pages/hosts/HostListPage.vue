<template>
  <q-page padding>
    <div class="row items-center q-mb-lg">
      <div class="text-h5 text-weight-medium">Hosts</div>
      <q-space />
      <q-btn
        color="primary"
        icon="add"
        label="Add Host"
        no-caps
        unelevated
        :to="{ name: 'host-add' }"
      />
    </div>

    <q-card v-if="loading" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-spinner-dots color="primary" size="40px" />
        <div class="text-grey q-mt-sm">Loading hosts...</div>
      </q-card-section>
    </q-card>

    <q-card v-else-if="error" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-icon name="error_outline" color="negative" size="48px" />
        <div class="text-negative q-mt-sm">{{ error }}</div>
        <q-btn
          class="q-mt-md"
          color="primary"
          label="Retry"
          no-caps
          outline
          @click="fetchHosts"
        />
      </q-card-section>
    </q-card>

    <q-card v-else-if="hosts.length === 0" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-icon name="dns" color="grey-6" size="64px" />
        <div class="text-h6 text-grey-5 q-mt-md">No hosts configured</div>
        <div class="text-grey q-mt-xs">Add a storage backend host to get started.</div>
        <q-btn
          class="q-mt-lg"
          color="primary"
          icon="add"
          label="Add Host"
          no-caps
          unelevated
          :to="{ name: 'host-add' }"
        />
      </q-card-section>
    </q-card>

    <div v-else class="row q-col-gutter-md">
      <div
        v-for="host in hosts"
        :key="`${host.namespace}/${host.name}`"
        class="col-12 col-sm-6 col-lg-4"
      >
        <q-card
          flat
          bordered
          class="cryo-card cryo-card--clickable cursor-pointer"
          @click="goToDetail(host)"
        >
          <q-card-section>
            <div class="row items-center no-wrap q-mb-sm">
              <q-icon :name="hostIcon(host.type)" color="primary" size="24px" class="q-mr-sm" />
              <div class="text-subtitle1 text-weight-medium ellipsis">{{ host.name }}</div>
            </div>
            <div class="row items-center q-gutter-x-sm">
              <q-badge :color="typeColor(host.type)" :label="host.type" />
              <span class="text-caption text-grey">{{ host.namespace }}</span>
            </div>
            <div class="text-caption text-grey-6 q-mt-sm ellipsis">{{ host.baseUrl }}</div>
          </q-card-section>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import hostsApi, { type RepositoryHost, type HostType } from 'src/api/hosts';

const router = useRouter();
const hosts = ref<RepositoryHost[]>([]);
const loading = ref(true);
const error = ref('');

function hostIcon(type: HostType): string {
  switch (type) {
    case 's3': return 'cloud';
    case 'sftp': return 'dns';
    case 'rest': return 'language';
    case 'local': return 'folder';
    default: return 'dns';
  }
}

function typeColor(type: HostType): string {
  switch (type) {
    case 's3': return 'orange';
    case 'sftp': return 'purple';
    case 'rest': return 'blue';
    case 'local': return 'green';
    default: return 'grey';
  }
}

function goToDetail(host: RepositoryHost) {
  router.push({ name: 'host-detail', params: { namespace: host.namespace, name: host.name } });
}

async function fetchHosts() {
  loading.value = true;
  error.value = '';
  try {
    hosts.value = await hostsApi.listHosts();
  } catch (e) {
    error.value = 'Failed to load hosts.';
    console.error('Failed to fetch hosts:', e);
  } finally {
    loading.value = false;
  }
}

onMounted(fetchHosts);
</script>

<template>
  <q-page padding>
    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading host...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ error }}</div>
      <q-btn class="q-mt-md" color="primary" label="Back" no-caps outline :to="{ name: 'hosts' }" />
    </div>

    <template v-else-if="host">
      <!-- Header -->
      <div class="row items-center q-mb-lg">
        <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'hosts' }" class="q-mr-sm" />
        <q-icon :name="hostIcon(host.type)" color="primary" size="28px" class="q-mr-sm" />
        <div>
          <div class="text-h5 text-weight-medium">{{ host.name }}</div>
          <div class="text-caption text-grey">{{ host.namespace }}</div>
        </div>
        <q-space />
        <q-btn
          color="primary"
          label="Edit"
          icon="edit"
          no-caps
          outline
          :to="{ name: 'host-edit', params: { namespace, name } }"
          class="q-mr-sm"
        />
        <q-btn
          color="negative"
          label="Delete"
          icon="delete"
          no-caps
          outline
          @click="showDeleteDialog = true"
        />
      </div>

      <!-- Delete confirmation dialog -->
      <q-dialog v-model="showDeleteDialog">
        <q-card style="min-width: 350px;">
          <q-card-section>
            <div class="text-h6">Delete Host</div>
          </q-card-section>
          <q-card-section>
            Are you sure you want to delete <strong>{{ host.name }}</strong>? This action cannot be undone.
          </q-card-section>
          <q-card-actions align="right">
            <q-btn flat label="Cancel" no-caps v-close-popup />
            <q-btn flat label="Delete" color="negative" no-caps :loading="deleting" @click="onDelete" />
          </q-card-actions>
        </q-card>
      </q-dialog>

      <!-- Delete error banner -->
      <q-banner v-if="deleteError" class="bg-negative text-white q-mb-md" rounded>
        <template v-slot:avatar>
          <q-icon name="error" />
        </template>
        {{ deleteError }}
      </q-banner>

      <!-- Info card -->
      <q-card flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="text-subtitle2 text-grey q-mb-sm">Host Details</div>
          <q-list>
            <q-item>
              <q-item-section avatar><q-icon name="label" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Type</q-item-label>
                <q-item-label><q-badge :color="typeColor(host.type)" :label="host.type" /></q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="link" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Base URL</q-item-label>
                <q-item-label class="text-mono">{{ host.baseUrl }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
      </q-card>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { AxiosError } from 'axios';
import hostsApi, { type RepositoryHost, type HostType } from 'src/api/hosts';

const route = useRoute();
const router = useRouter();
const namespace = route.params.namespace as string;
const name = route.params.name as string;

const host = ref<RepositoryHost | null>(null);
const loading = ref(true);
const error = ref('');

const showDeleteDialog = ref(false);
const deleting = ref(false);
const deleteError = ref('');

function hostIcon(type: HostType): string {
  switch (type) {
    case 's3': return 'cloud';
    case 'sftp': return 'dns';
    case 'rest': return 'language';
    case 'local': return 'folder';
    default: return 'dns';
  }
}

async function onDelete() {
  deleting.value = true;
  deleteError.value = '';
  try {
    await hostsApi.deleteHost(namespace, name);
    router.push({ name: 'hosts' });
  } catch (e: unknown) {
    const axiosErr = e as AxiosError<{ error?: string; repositories?: string[] }>;
    if (axiosErr.response?.status === 409) {
      const repos = axiosErr.response.data?.repositories ?? [];
      deleteError.value = `Cannot delete: host is referenced by ${repos.length} repository(ies): ${repos.join(', ')}`;
    } else {
      deleteError.value = 'Failed to delete host.';
    }
    console.error('Failed to delete host:', e);
  } finally {
    deleting.value = false;
    showDeleteDialog.value = false;
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

onMounted(async () => {
  try {
    host.value = await hostsApi.getHost(namespace, name);
  } catch (e) {
    error.value = 'Host not found.';
    console.error('Failed to fetch host:', e);
  } finally {
    loading.value = false;
  }
});
</script>

<style lang="scss" scoped>
.text-mono {
  font-family: 'Roboto Mono', monospace;
  font-size: 0.85em;
}
</style>

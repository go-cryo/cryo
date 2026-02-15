<template>
  <q-page padding>
    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading snapshot files...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ error }}</div>
      <q-btn
        class="q-mt-md"
        color="primary"
        label="Back to Repository"
        no-caps
        outline
        :to="{ name: 'repository-detail', params: { namespace, name } }"
      />
    </div>

    <template v-else>
      <!-- Header -->
      <div class="row items-center q-mb-md">
        <q-btn
          flat
          round
          icon="arrow_back"
          color="grey"
          :to="{ name: 'repository-detail', params: { namespace, name } }"
          class="q-mr-sm"
        />
        <q-icon name="inventory_2" color="primary" size="28px" class="q-mr-sm" />
        <div>
          <div class="text-h5 text-weight-medium text-mono">{{ snapshotId.substring(0, 8) }}</div>
          <div class="text-caption text-grey">{{ namespace }}/{{ name }}</div>
        </div>
      </div>

      <!-- Breadcrumbs -->
      <q-card flat bordered class="cryo-card q-mb-md">
        <q-card-section class="q-py-sm">
          <q-breadcrumbs>
            <q-breadcrumbs-el
              label="/"
              class="cursor-pointer text-primary"
              @click="navigateTo('/')"
            />
            <q-breadcrumbs-el
              v-for="(segment, idx) in pathSegments"
              :key="idx"
              :label="segment.name"
              class="cursor-pointer text-primary"
              @click="navigateTo(segment.path)"
            />
          </q-breadcrumbs>
        </q-card-section>
      </q-card>

      <!-- File table -->
      <q-card flat bordered class="cryo-card">
        <q-card-section v-if="browseData && browseData.entries.length === 0" class="text-center text-grey">
          This directory is empty.
        </q-card-section>

        <q-table
          v-else-if="browseData"
          flat
          dense
          :rows="sortedEntries"
          :columns="columns"
          row-key="path"
          :pagination="{ rowsPerPage: 0 }"
          hide-pagination
          class="cryo-browse-table"
        >
          <template #body="props">
            <q-tr
              :props="props"
              :class="props.row.type === 'dir' ? 'cursor-pointer cryo-clickable-row' : ''"
              @click="onRowClick(props.row)"
            >
              <q-td key="name" :props="props">
                <q-icon
                  :name="props.row.type === 'dir' ? 'folder' : 'description'"
                  :color="props.row.type === 'dir' ? 'primary' : 'grey'"
                  size="20px"
                  class="q-mr-sm"
                />
                <span :class="props.row.type === 'dir' ? 'text-primary text-weight-medium' : ''">
                  {{ props.row.name }}
                </span>
              </q-td>
              <q-td key="size" :props="props">
                {{ props.row.type === 'file' ? formatSize(props.row.size) : '-' }}
              </q-td>
              <q-td key="mtime" :props="props">
                {{ formatDate(props.row.mtime) }}
              </q-td>
            </q-tr>
          </template>
        </q-table>
      </q-card>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { QTableColumn } from 'quasar';
import repositoriesApi, { type SnapshotBrowseResponse, type SnapshotEntry } from 'src/api/repositories';

const route = useRoute();
const router = useRouter();

const namespace = route.params.namespace as string;
const name = route.params.name as string;
const snapshotId = route.params.snapshotId as string;

const currentPath = computed(() => (route.query.path as string) || '/');

const browseData = ref<SnapshotBrowseResponse | null>(null);
const loading = ref(true);
const error = ref('');

const columns: QTableColumn[] = [
  { name: 'name', label: 'Name', field: 'name', align: 'left', sortable: true },
  { name: 'size', label: 'Size', field: 'size', align: 'left', sortable: true },
  { name: 'mtime', label: 'Modified', field: 'mtime', align: 'left', sortable: true },
];

const pathSegments = computed(() => {
  const p = currentPath.value;
  if (p === '/') return [];
  const parts = p.split('/').filter(Boolean);
  return parts.map((part, idx) => ({
    name: part,
    path: '/' + parts.slice(0, idx + 1).join('/'),
  }));
});

const sortedEntries = computed(() => {
  if (!browseData.value) return [];
  const entries = [...browseData.value.entries];
  entries.sort((a, b) => {
    // Directories first
    if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
    // Then alphabetical
    return a.name.localeCompare(b.name);
  });
  return entries;
});

function navigateTo(path: string) {
  router.replace({ query: { path } });
}

function onRowClick(entry: SnapshotEntry) {
  if (entry.type === 'dir') {
    navigateTo(entry.path);
  }
}

function formatSize(bytes: number | undefined | null): string {
  if (bytes == null || bytes === 0) return '-';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let i = 0;
  let size = bytes;
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024;
    i++;
  }
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDate(iso: string): string {
  if (!iso) return '-';
  return new Date(iso).toLocaleString();
}

async function fetchFiles() {
  loading.value = true;
  error.value = '';
  try {
    browseData.value = await repositoriesApi.browseSnapshot(namespace, name, snapshotId, currentPath.value);
  } catch (e) {
    error.value = 'Failed to load snapshot files.';
    console.error('Failed to browse snapshot:', e);
  } finally {
    loading.value = false;
  }
}

watch(currentPath, () => {
  fetchFiles();
});

onMounted(() => {
  fetchFiles();
});
</script>

<style lang="scss" scoped>
.text-mono {
  font-family: 'Roboto Mono', monospace;
  font-size: 0.85em;
}

.cryo-clickable-row:hover {
  background: rgba(0, 0, 0, 0.03);
}
</style>

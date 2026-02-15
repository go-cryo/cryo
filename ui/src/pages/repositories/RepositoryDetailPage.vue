<template>
  <q-page padding>
    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading repository...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ error }}</div>
      <q-btn class="q-mt-md" color="primary" label="Back" no-caps outline :to="{ name: 'repositories' }" />
    </div>

    <template v-else-if="repo">
      <!-- Header -->
      <div class="row items-center q-mb-lg">
        <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'repositories' }" class="q-mr-sm" />
        <q-icon :name="repoIcon(repo.type)" color="primary" size="28px" class="q-mr-sm" />
        <div>
          <div class="text-h5 text-weight-medium">{{ repo.name }}</div>
          <div class="text-caption text-grey">{{ repo.namespace }}</div>
        </div>
        <q-space />
        <q-btn
          color="primary"
          label="Edit"
          icon="edit"
          no-caps
          outline
          :to="{ name: 'repository-edit', params: { namespace, name } }"
          class="q-mr-sm"
        />
        <q-btn
          color="primary"
          label="Check Health"
          icon="monitor_heart"
          no-caps
          outline
          :loading="checking"
          @click="checkHealth"
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
            <div class="text-h6">Delete Repository</div>
          </q-card-section>
          <q-card-section>
            Are you sure you want to delete <strong>{{ repo.name }}</strong>? This action cannot be undone.
          </q-card-section>
          <q-card-actions align="right">
            <q-btn flat label="Cancel" no-caps v-close-popup />
            <q-btn flat label="Delete" color="negative" no-caps :loading="deleting" @click="onDelete" />
          </q-card-actions>
        </q-card>
      </q-dialog>

      <!-- Info card -->
      <q-card flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="text-subtitle2 text-grey q-mb-sm">Repository Details</div>
          <q-list>
            <q-item>
              <q-item-section avatar><q-icon name="label" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Type</q-item-label>
                <q-item-label><q-badge :color="typeColor(repo.type)" :label="repo.type" /></q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="repo.hostRef">
              <q-item-section avatar><q-icon name="dns" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Host</q-item-label>
                <q-item-label>
                  <router-link
                    :to="{ name: 'host-detail', params: hostRefParams }"
                    class="text-primary"
                  >{{ repo.hostRef }}</router-link>
                </q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="repo.path">
              <q-item-section avatar><q-icon name="folder" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Path</q-item-label>
                <q-item-label class="text-mono">{{ repo.path }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="link" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>URL</q-item-label>
                <q-item-label class="text-mono">{{ repo.url }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
      </q-card>

      <!-- Health status -->
      <q-card v-if="healthStatus" flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="row items-center q-gutter-sm">
            <q-icon
              :name="healthStatus.ok ? 'check_circle' : 'error'"
              :color="healthStatus.ok ? 'positive' : 'negative'"
              size="24px"
            />
            <span class="text-subtitle2">{{ healthStatus.ok ? 'Repository is healthy' : 'Repository check failed' }}</span>
          </div>
          <div v-if="healthStatus.message" class="text-caption text-grey q-mt-xs text-mono">{{ healthStatus.message }}</div>
        </q-card-section>
      </q-card>

      <!-- Snapshots -->
      <q-card flat bordered class="cryo-card">
        <q-card-section>
          <div class="row items-center q-mb-sm">
            <div class="text-subtitle2 text-grey">Snapshots</div>
            <q-space />
            <q-btn
              flat
              dense
              icon="refresh"
              color="grey"
              :loading="loadingSnapshots"
              @click="fetchSnapshots"
            />
          </div>
        </q-card-section>

        <q-card-section v-if="loadingSnapshots" class="text-center">
          <q-spinner-dots color="primary" size="32px" />
        </q-card-section>

        <q-card-section v-else-if="snapshotError" class="text-center">
          <div class="text-negative">{{ snapshotError }}</div>
        </q-card-section>

        <q-card-section v-else-if="snapshots.length === 0" class="text-center text-grey">
          No snapshots found.
        </q-card-section>

        <q-table
          v-else
          flat
          dense
          :rows="snapshots"
          :columns="snapshotColumns"
          row-key="id"
          v-model:pagination="snapshotPagination"
          :rows-per-page-options="[10, 25, 50, 0]"
          class="cryo-snapshot-table"
          @row-click="(_evt: Event, row: Snapshot) => onSnapshotClick(row)"
        >
          <template #body-cell-id="props">
            <q-td :props="props" class="text-mono">
              {{ props.row.id.substring(0, 8) }}
              <q-btn
                flat
                dense
                round
                size="xs"
                icon="content_copy"
                color="grey"
                class="q-ml-xs"
                @click.stop="copyToClipboard(props.row.id)"
              />
            </q-td>
          </template>
          <template #body-cell-time="props">
            <q-td :props="props">
              {{ formatDate(props.row.time) }}
              <span class="text-grey q-ml-xs">({{ formatRelativeTime(props.row.time) }})</span>
            </q-td>
          </template>
          <template #body-cell-size="props">
            <q-td :props="props">
              {{ formatSize(props.row.summary?.total_bytes_processed) }}
            </q-td>
          </template>
          <template #body-cell-tags="props">
            <q-td :props="props">
              <q-badge
                v-for="tag in (props.row.tags || [])"
                :key="tag"
                outline
                color="primary"
                :label="tag"
                class="q-mr-xs"
              />
            </q-td>
          </template>
          <template #body-cell-browse="props">
            <q-td :props="props">
              <q-btn
                flat
                dense
                round
                icon="folder_open"
                color="primary"
                size="sm"
                @click.stop="onSnapshotClick(props.row)"
              >
                <q-tooltip>Browse files</q-tooltip>
              </q-btn>
            </q-td>
          </template>
        </q-table>
      </q-card>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { copyToClipboard as qtCopy, type QTableColumn } from 'quasar';
import repositoriesApi, { type Repository, type RepositoryType, type RepositoryStatus, type Snapshot } from 'src/api/repositories';

const route = useRoute();
const router = useRouter();
const namespace = route.params.namespace as string;
const name = route.params.name as string;

const repo = ref<Repository | null>(null);
const loading = ref(true);
const error = ref('');

const healthStatus = ref<RepositoryStatus | null>(null);
const checking = ref(false);

const snapshots = ref<Snapshot[]>([]);
const loadingSnapshots = ref(false);
const snapshotError = ref('');

const showDeleteDialog = ref(false);
const deleting = ref(false);

const snapshotColumns: QTableColumn[] = [
  {
    name: 'id',
    label: 'ID',
    field: 'id',
    align: 'left',
    sortable: true,
  },
  {
    name: 'time',
    label: 'Created',
    field: 'time',
    align: 'left',
    sortable: true,
    sort: (a: string, b: string) => new Date(a).getTime() - new Date(b).getTime(),
  },
  {
    name: 'size',
    label: 'Size',
    field: (row: Snapshot) => row.summary?.total_bytes_processed ?? 0,
    align: 'left',
    sortable: true,
  },
  {
    name: 'hostname',
    label: 'Host',
    field: 'hostname',
    align: 'left',
    sortable: true,
  },
  {
    name: 'tags',
    label: 'Tags',
    field: 'tags',
    align: 'left',
    sortable: false,
  },
  {
    name: 'browse',
    label: '',
    field: 'id',
    align: 'right',
    sortable: false,
  },
];

const snapshotPagination = ref({
  sortBy: 'time',
  descending: true,
  page: 1,
  rowsPerPage: 25,
});

const hostRefParams = computed(() => {
  if (!repo.value?.hostRef) return {};
  const parts = repo.value.hostRef.split('/');
  return parts.length === 2 ? { namespace: parts[0], name: parts[1] } : {};
});

function repoIcon(type: RepositoryType): string {
  switch (type) {
    case 's3': return 'cloud';
    case 'sftp': return 'dns';
    case 'rest': return 'language';
    case 'local': return 'folder';
    default: return 'inventory_2';
  }
}

function typeColor(type: RepositoryType): string {
  switch (type) {
    case 's3': return 'orange';
    case 'sftp': return 'purple';
    case 'rest': return 'blue';
    case 'local': return 'green';
    default: return 'grey';
  }
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

function formatRelativeTime(iso: string): string {
  const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  const years = Math.floor(days / 365);
  return `${years}y ago`;
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

function onSnapshotClick(row: Snapshot) {
  router.push({
    name: 'snapshot-browse',
    params: { namespace, name, snapshotId: row.id },
  });
}

function copyToClipboard(text: string) {
  qtCopy(text).catch(() => {
    console.error('Failed to copy to clipboard');
  });
}

async function onDelete() {
  deleting.value = true;
  try {
    await repositoriesApi.deleteRepository(namespace, name);
    router.push({ name: 'repositories' });
  } catch (e) {
    console.error('Failed to delete repository:', e);
  } finally {
    deleting.value = false;
    showDeleteDialog.value = false;
  }
}

async function checkHealth() {
  checking.value = true;
  try {
    healthStatus.value = await repositoriesApi.checkRepository(namespace, name);
  } catch (e) {
    healthStatus.value = { ok: false, message: 'Failed to reach the repository.' };
    console.error('Health check failed:', e);
  } finally {
    checking.value = false;
  }
}

async function fetchSnapshots() {
  loadingSnapshots.value = true;
  snapshotError.value = '';
  try {
    snapshots.value = await repositoriesApi.listSnapshots(namespace, name);
  } catch (e) {
    snapshotError.value = 'Failed to load snapshots.';
    console.error('Failed to fetch snapshots:', e);
  } finally {
    loadingSnapshots.value = false;
  }
}

onMounted(async () => {
  try {
    repo.value = await repositoriesApi.getRepository(namespace, name);
  } catch (e) {
    error.value = 'Repository not found.';
    console.error('Failed to fetch repository:', e);
  } finally {
    loading.value = false;
  }
  fetchSnapshots();
});
</script>

<style lang="scss" scoped>
.text-mono {
  font-family: 'Roboto Mono', monospace;
  font-size: 0.85em;
}

.cryo-snapshot-table :deep(tbody tr) {
  cursor: pointer;

  &:hover {
    background: rgba(0, 0, 0, 0.03);
  }
}
</style>

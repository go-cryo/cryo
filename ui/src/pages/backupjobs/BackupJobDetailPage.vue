<template>
  <q-page padding>
    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading backup job...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ error }}</div>
      <q-btn class="q-mt-md" color="primary" label="Back" no-caps outline :to="{ name: 'backupjobs' }" />
    </div>

    <template v-else-if="job">
      <!-- Header -->
      <div class="row items-center q-mb-lg">
        <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'backupjobs' }" class="q-mr-sm" />
        <q-icon :name="jobIcon(job.type)" color="primary" size="28px" class="q-mr-sm" />
        <div>
          <div class="text-h5 text-weight-medium">{{ job.name }}</div>
          <div class="text-caption text-grey">{{ job.namespace }}</div>
        </div>
        <q-space />
        <q-btn
          color="primary"
          label="Run Now"
          icon="play_arrow"
          no-caps
          unelevated
          :loading="triggering"
          @click="onTrigger"
          class="q-mr-sm"
        />
        <q-btn
          color="primary"
          label="Edit"
          icon="edit"
          no-caps
          outline
          :to="{ name: 'backupjob-edit', params: { namespace, name } }"
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

      <!-- Trigger result banner -->
      <q-banner v-if="triggerResult" class="bg-positive text-white q-mb-md" rounded>
        <template v-slot:avatar>
          <q-icon name="check_circle" />
        </template>
        Backup run <strong>{{ triggerResult.name }}</strong> started.
      </q-banner>
      <q-banner v-if="triggerError" class="bg-negative text-white q-mb-md" rounded>
        <template v-slot:avatar>
          <q-icon name="error" />
        </template>
        {{ triggerError }}
      </q-banner>

      <!-- Delete confirmation dialog -->
      <q-dialog v-model="showDeleteDialog">
        <q-card style="min-width: 350px;">
          <q-card-section>
            <div class="text-h6">Delete Backup Job</div>
          </q-card-section>
          <q-card-section>
            Are you sure you want to delete <strong>{{ job.name }}</strong>? This action cannot be undone.
          </q-card-section>
          <q-card-actions align="right">
            <q-btn flat label="Cancel" no-caps v-close-popup />
            <q-btn flat label="Delete" color="negative" no-caps :loading="deleting" @click="onDelete" />
          </q-card-actions>
        </q-card>
      </q-dialog>

      <!-- Details card -->
      <q-card flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="text-subtitle2 text-grey q-mb-sm">Job Details</div>
          <q-list>
            <q-item>
              <q-item-section avatar><q-icon name="label" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Type</q-item-label>
                <q-item-label><q-badge :color="typeColor(job.type)" :label="job.type" /></q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="schedule" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Schedule</q-item-label>
                <q-item-label class="text-mono">{{ job.schedule }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="inventory_2" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Repository</q-item-label>
                <q-item-label>
                  <router-link
                    :to="{ name: 'repository-detail', params: repoRefParams }"
                    class="text-primary"
                  >{{ job.repositoryRef }}</router-link>
                </q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="pause_circle" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Suspended</q-item-label>
                <q-item-label>
                  <q-badge :color="job.suspend ? 'warning' : 'positive'" :label="job.suspend ? 'Yes' : 'No'" />
                </q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.nextRun">
              <q-item-section avatar><q-icon name="event" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Next Run</q-item-label>
                <q-item-label>{{ formatDate(job.nextRun) }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.image">
              <q-item-section avatar><q-icon name="image" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Image</q-item-label>
                <q-item-label class="text-mono">{{ job.image }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
      </q-card>

      <!-- Type-specific config card -->
      <q-card v-if="job.psql" flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="text-subtitle2 text-grey q-mb-sm">PostgreSQL Configuration</div>
          <q-list>
            <q-item>
              <q-item-section avatar><q-icon name="dns" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Hostname</q-item-label>
                <q-item-label class="text-mono">{{ job.psql.hostname }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="tag" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Port</q-item-label>
                <q-item-label class="text-mono">{{ job.psql.port ?? 5432 }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="person" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Username</q-item-label>
                <q-item-label class="text-mono">{{ job.psql.username }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="storage" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Database</q-item-label>
                <q-item-label class="text-mono">{{ job.psql.database }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.psql.credentialSecretRef">
              <q-item-section avatar><q-icon name="vpn_key" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Credentials Secret</q-item-label>
                <q-item-label class="text-mono">{{ job.psql.credentialSecretRef }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
      </q-card>

      <q-card v-if="job.s3" flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="text-subtitle2 text-grey q-mb-sm">S3 Configuration</div>
          <q-list>
            <q-item>
              <q-item-section avatar><q-icon name="cloud" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Endpoint</q-item-label>
                <q-item-label class="text-mono">{{ job.s3.endpoint }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="folder" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Bucket</q-item-label>
                <q-item-label class="text-mono">{{ job.s3.bucket }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section avatar><q-icon name="vpn_key" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Credentials Secret</q-item-label>
                <q-item-label class="text-mono">{{ job.s3.credentialsSecretRef.name }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
      </q-card>

      <q-card v-if="job.pvc" flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="text-subtitle2 text-grey q-mb-sm">PVC Configuration</div>
          <q-list>
            <q-item>
              <q-item-section avatar><q-icon name="save" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Claim Name</q-item-label>
                <q-item-label class="text-mono">{{ job.pvc.claimName }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.pvc.volumeSnapshotClassName">
              <q-item-section avatar><q-icon name="photo_camera" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Volume Snapshot Class</q-item-label>
                <q-item-label class="text-mono">{{ job.pvc.volumeSnapshotClassName }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.pvc.snapshotRetention != null">
              <q-item-section avatar><q-icon name="history" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Snapshot Retention</q-item-label>
                <q-item-label>{{ job.pvc.snapshotRetention }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
      </q-card>

      <!-- Retention policy card -->
      <q-card v-if="job.retention" flat bordered class="cryo-card q-mb-md">
        <q-card-section>
          <div class="text-subtitle2 text-grey q-mb-sm">Retention Policy</div>
          <q-list>
            <q-item v-if="job.retention.keepLast != null">
              <q-item-section avatar><q-icon name="filter_list" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Keep Last</q-item-label>
                <q-item-label>{{ job.retention.keepLast }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.retention.keepDaily != null">
              <q-item-section avatar><q-icon name="today" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Keep Daily</q-item-label>
                <q-item-label>{{ job.retention.keepDaily }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.retention.keepWeekly != null">
              <q-item-section avatar><q-icon name="date_range" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Keep Weekly</q-item-label>
                <q-item-label>{{ job.retention.keepWeekly }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="job.retention.keepMonthly != null">
              <q-item-section avatar><q-icon name="calendar_month" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label caption>Keep Monthly</q-item-label>
                <q-item-label>{{ job.retention.keepMonthly }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
      </q-card>

      <!-- Runs table -->
      <q-card flat bordered class="cryo-card">
        <q-card-section>
          <div class="row items-center q-mb-sm">
            <div class="text-subtitle2 text-grey">Backup Runs</div>
            <q-space />
            <q-btn
              flat
              dense
              icon="refresh"
              color="grey"
              :loading="loadingRuns"
              @click="fetchRuns"
            />
          </div>
        </q-card-section>

        <q-card-section v-if="loadingRuns" class="text-center">
          <q-spinner-dots color="primary" size="32px" />
        </q-card-section>

        <q-card-section v-else-if="runsError" class="text-center">
          <div class="text-negative">{{ runsError }}</div>
        </q-card-section>

        <q-card-section v-else-if="runs.length === 0" class="text-center text-grey">
          No backup runs found.
        </q-card-section>

        <q-table
          v-else
          flat
          dense
          :rows="runs"
          :columns="runColumns"
          row-key="name"
          v-model:pagination="runPagination"
          :rows-per-page-options="[10, 25, 50, 0]"
        >
          <template #body-cell-status="props">
            <q-td :props="props">
              <q-badge :color="runStatusColor(props.row.status)" :label="props.row.status" />
            </q-td>
          </template>
          <template #body-cell-startTime="props">
            <q-td :props="props">
              {{ props.row.startTime ? formatDate(props.row.startTime) : '-' }}
            </q-td>
          </template>
          <template #body-cell-endTime="props">
            <q-td :props="props">
              {{ props.row.endTime ? formatDate(props.row.endTime) : '-' }}
            </q-td>
          </template>
          <template #body-cell-duration="props">
            <q-td :props="props">
              {{ formatDuration(props.row.startTime, props.row.endTime) }}
            </q-td>
          </template>
          <template #body-cell-name="props">
            <q-td :props="props" class="text-mono">
              {{ props.row.name }}
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
import type { QTableColumn } from 'quasar';
import backupJobsApi, { type BackupJob, type BackupJobType, type BackupRunStatus, type BackupRun } from 'src/api/backupjobs';

const route = useRoute();
const router = useRouter();
const namespace = route.params.namespace as string;
const name = route.params.name as string;

const job = ref<BackupJob | null>(null);
const loading = ref(true);
const error = ref('');

const runs = ref<BackupRun[]>([]);
const loadingRuns = ref(false);
const runsError = ref('');

const showDeleteDialog = ref(false);
const deleting = ref(false);

const triggering = ref(false);
const triggerResult = ref<BackupRun | null>(null);
const triggerError = ref('');

const runColumns: QTableColumn[] = [
  {
    name: 'status',
    label: 'Status',
    field: 'status',
    align: 'left',
    sortable: true,
  },
  {
    name: 'startTime',
    label: 'Start Time',
    field: 'startTime',
    align: 'left',
    sortable: true,
    sort: (a: string, b: string) => new Date(a).getTime() - new Date(b).getTime(),
  },
  {
    name: 'endTime',
    label: 'End Time',
    field: 'endTime',
    align: 'left',
    sortable: true,
  },
  {
    name: 'duration',
    label: 'Duration',
    field: 'startTime',
    align: 'left',
    sortable: false,
  },
  {
    name: 'name',
    label: 'Name',
    field: 'name',
    align: 'left',
    sortable: true,
  },
];

const runPagination = ref({
  sortBy: 'startTime',
  descending: true,
  page: 1,
  rowsPerPage: 25,
});

const repoRefParams = computed(() => {
  if (!job.value?.repositoryRef) return {};
  const parts = job.value.repositoryRef.split('/');
  return parts.length === 2 ? { namespace: parts[0], name: parts[1] } : {};
});

function jobIcon(type: BackupJobType): string {
  switch (type) {
    case 'psql': return 'storage';
    case 's3': return 'cloud';
    case 'pvc': return 'save';
    default: return 'schedule';
  }
}

function typeColor(type: BackupJobType): string {
  switch (type) {
    case 'psql': return 'teal';
    case 's3': return 'orange';
    case 'pvc': return 'indigo';
    default: return 'grey';
  }
}

function runStatusColor(status: BackupRunStatus): string {
  switch (status) {
    case 'Running': return 'blue';
    case 'Succeeded': return 'positive';
    case 'Failed': return 'negative';
    default: return 'grey';
  }
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

function formatDuration(start?: string, end?: string): string {
  if (!start || !end) return '-';
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 0) return '-';
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}h ${remainingMinutes}m`;
}

async function onDelete() {
  deleting.value = true;
  try {
    await backupJobsApi.deleteBackupJob(namespace, name);
    router.push({ name: 'backupjobs' });
  } catch (e) {
    console.error('Failed to delete backup job:', e);
  } finally {
    deleting.value = false;
    showDeleteDialog.value = false;
  }
}

async function onTrigger() {
  triggering.value = true;
  triggerResult.value = null;
  triggerError.value = '';
  try {
    triggerResult.value = await backupJobsApi.triggerBackupJob(namespace, name);
    fetchRuns();
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    triggerError.value = `Failed to trigger backup: ${msg}`;
    console.error('Failed to trigger backup:', e);
  } finally {
    triggering.value = false;
  }
}

async function fetchRuns() {
  loadingRuns.value = true;
  runsError.value = '';
  try {
    runs.value = await backupJobsApi.listBackupJobRuns(namespace, name);
  } catch (e) {
    runsError.value = 'Failed to load backup runs.';
    console.error('Failed to fetch backup runs:', e);
  } finally {
    loadingRuns.value = false;
  }
}

onMounted(async () => {
  try {
    job.value = await backupJobsApi.getBackupJob(namespace, name);
  } catch (e) {
    error.value = 'Backup job not found.';
    console.error('Failed to fetch backup job:', e);
  } finally {
    loading.value = false;
  }
  fetchRuns();
});
</script>

<style lang="scss" scoped>
.text-mono {
  font-family: 'Roboto Mono', monospace;
  font-size: 0.85em;
}
</style>

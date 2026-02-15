<template>
  <q-page padding>
    <div class="row items-center q-mb-lg">
      <div class="text-h5 text-weight-medium">Backup Jobs</div>
      <q-space />
      <q-btn
        color="primary"
        icon="add"
        label="Add Backup Job"
        no-caps
        unelevated
        :to="{ name: 'backupjob-add' }"
      />
    </div>

    <q-card v-if="loading" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-spinner-dots color="primary" size="40px" />
        <div class="text-grey q-mt-sm">Loading backup jobs...</div>
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
          @click="fetchBackupJobs"
        />
      </q-card-section>
    </q-card>

    <q-card v-else-if="backupJobs.length === 0" flat bordered class="cryo-card">
      <q-card-section class="text-center q-pa-xl">
        <q-icon name="schedule" color="grey-6" size="64px" />
        <div class="text-h6 text-grey-5 q-mt-md">No backup jobs configured</div>
        <div class="text-grey q-mt-xs">Add a backup job to get started.</div>
        <q-btn
          class="q-mt-lg"
          color="primary"
          icon="add"
          label="Add Backup Job"
          no-caps
          unelevated
          :to="{ name: 'backupjob-add' }"
        />
      </q-card-section>
    </q-card>

    <div v-else class="row q-col-gutter-md">
      <div
        v-for="job in backupJobs"
        :key="`${job.namespace}/${job.name}`"
        class="col-12 col-sm-6 col-lg-4"
      >
        <q-card
          flat
          bordered
          class="cryo-card cryo-card--clickable cursor-pointer"
          @click="goToDetail(job)"
        >
          <q-card-section>
            <div class="row items-center no-wrap q-mb-sm">
              <q-icon :name="jobIcon(job.type)" color="primary" size="24px" class="q-mr-sm" />
              <div class="text-subtitle1 text-weight-medium ellipsis">{{ job.name }}</div>
            </div>
            <div class="row items-center q-gutter-x-sm">
              <q-badge :color="typeColor(job.type)" :label="job.type" />
              <span class="text-caption text-grey">{{ job.namespace }}</span>
              <q-badge v-if="job.suspend" color="grey" label="Suspended" />
            </div>
            <div class="text-caption text-grey-6 q-mt-sm text-mono">{{ job.schedule }}</div>
            <div v-if="job.lastRun" class="q-mt-xs">
              <q-badge :color="runStatusColor(job.lastRun.status)" :label="job.lastRun.status" />
            </div>
          </q-card-section>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import backupJobsApi, { type BackupJob, type BackupJobType, type BackupRunStatus } from 'src/api/backupjobs';

const router = useRouter();
const backupJobs = ref<BackupJob[]>([]);
const loading = ref(true);
const error = ref('');

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

function goToDetail(job: BackupJob) {
  router.push({ name: 'backupjob-detail', params: { namespace: job.namespace, name: job.name } });
}

async function fetchBackupJobs() {
  loading.value = true;
  error.value = '';
  try {
    backupJobs.value = await backupJobsApi.listBackupJobs();
  } catch (e) {
    error.value = 'Failed to load backup jobs.';
    console.error('Failed to fetch backup jobs:', e);
  } finally {
    loading.value = false;
  }
}

onMounted(fetchBackupJobs);
</script>

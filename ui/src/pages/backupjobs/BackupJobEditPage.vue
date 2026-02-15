<template>
  <q-page padding>
    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading backup job...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="loadError" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ loadError }}</div>
      <q-btn class="q-mt-md" color="primary" label="Back" no-caps outline :to="{ name: 'backupjobs' }" />
    </div>

    <template v-else>
      <div class="row items-center q-mb-lg">
        <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'backupjob-detail', params: { namespace, name } }" class="q-mr-sm" />
        <div class="text-h5 text-weight-medium">Edit Backup Job</div>
      </div>

      <q-card flat bordered class="cryo-card" style="max-width: 600px;">
        <q-card-section>
          <q-form @submit.prevent="onSubmit" class="q-gutter-md">
            <!-- Name (read-only) -->
            <q-input
              :model-value="name"
              label="Name"
              outlined
              disable
            />

            <!-- Namespace (read-only) -->
            <q-input
              :model-value="namespace"
              label="Namespace"
              outlined
              disable
            />

            <!-- Type (read-only) -->
            <q-input
              :model-value="jobType"
              label="Backup Type"
              outlined
              disable
            />

            <!-- Schedule -->
            <q-input
              v-model="form.schedule"
              label="Schedule"
              hint="Cron expression, e.g. 0 2 * * *"
              outlined
              :rules="[val => !!val || 'Schedule is required']"
            />

            <!-- Repository ref -->
            <q-select
              v-model="form.repositoryRef"
              :options="repoOptions"
              label="Repository"
              outlined
              emit-value
              map-options
              :loading="loadingRepos"
              :rules="[val => !!val || 'Repository is required']"
            >
              <template v-slot:no-option>
                <q-item>
                  <q-item-section class="text-grey">
                    No repositories available.
                    <router-link :to="{ name: 'repository-add' }" class="text-primary">Create one first.</router-link>
                  </q-item-section>
                </q-item>
              </template>
            </q-select>

            <!-- Suspend -->
            <q-toggle
              v-model="form.suspend"
              label="Suspended"
            />

            <!-- Image override -->
            <q-input
              v-model="form.image"
              label="Image (optional)"
              hint="Override the default backup image"
              outlined
            />

            <!-- Retention Policy -->
            <div class="text-subtitle2 text-grey q-mt-md">Retention Policy (optional)</div>
            <div class="row q-col-gutter-sm">
              <div class="col-6">
                <q-input
                  v-model.number="form.retention.keepLast"
                  label="Keep Last"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
              <div class="col-6">
                <q-input
                  v-model.number="form.retention.keepDaily"
                  label="Keep Daily"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
              <div class="col-6">
                <q-input
                  v-model.number="form.retention.keepWeekly"
                  label="Keep Weekly"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
              <div class="col-6">
                <q-input
                  v-model.number="form.retention.keepMonthly"
                  label="Keep Monthly"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
            </div>

            <!-- PSQL Config -->
            <template v-if="jobType === 'psql'">
              <div class="text-subtitle2 text-grey q-mt-md">PostgreSQL Configuration</div>
              <q-input
                v-model="form.psql.hostname"
                label="Hostname"
                outlined
                :rules="[val => !!val || 'Hostname is required']"
              />
              <q-input
                v-model.number="form.psql.port"
                label="Port"
                outlined
                type="number"
                placeholder="5432"
              />
              <q-input
                v-model="form.psql.username"
                label="Username"
                outlined
                :rules="[val => !!val || 'Username is required']"
              />
              <q-input
                v-model="form.psql.database"
                label="Database"
                outlined
                :rules="[val => !!val || 'Database is required']"
              />
              <q-input
                v-model="form.psql.password"
                label="Password"
                hint="Leave empty to keep existing password"
                outlined
                type="password"
              />
              <div class="text-subtitle2 text-grey q-mt-md">Staging Storage (optional)</div>
              <q-input
                v-model="form.psql.stagingSize"
                label="Staging PVC Size"
                hint="e.g. 1Gi, 10Gi — default: 1Gi"
                outlined
              />
              <q-input
                v-model="form.psql.stagingStorageClassName"
                label="Storage Class Name"
                hint="Override default from settings"
                outlined
              />
            </template>

            <!-- S3 Config -->
            <template v-if="jobType === 's3'">
              <div class="text-subtitle2 text-grey q-mt-md">S3 Configuration</div>
              <q-input
                v-model="form.s3.endpoint"
                label="Endpoint"
                outlined
                :rules="[val => !!val || 'Endpoint is required']"
              />
              <q-input
                v-model="form.s3.bucket"
                label="Bucket"
                outlined
                :rules="[val => !!val || 'Bucket is required']"
              />
              <div class="text-subtitle2 text-grey q-mt-sm">Credentials</div>
              <q-btn-toggle
                v-model="s3CredentialMode"
                no-caps
                unelevated
                spread
                toggle-color="primary"
                :options="[
                  { label: 'Update credentials', value: 'inline' },
                  { label: 'Use existing secret', value: 'ref' },
                ]"
                class="q-mb-sm"
              />
              <template v-if="s3CredentialMode === 'inline'">
                <q-input
                  v-model="form.s3.accessKey"
                  label="Access Key"
                  outlined
                  :rules="[val => !!val || 'Access key is required']"
                />
                <q-input
                  v-model="form.s3.secretKey"
                  label="Secret Key"
                  outlined
                  type="password"
                  :rules="[val => !!val || 'Secret key is required']"
                />
              </template>
              <template v-else>
                <q-input
                  v-model="form.s3.credentialsSecretRef.name"
                  label="Secret Name"
                  hint="Name of the Kubernetes Secret containing S3 credentials"
                  outlined
                  :rules="[val => !!val || 'Secret name is required']"
                />
                <q-input
                  v-model="form.s3.credentialsSecretRef.accessKeyKey"
                  label="Access Key Key"
                  hint="Key within the Secret for the access key"
                  outlined
                  :rules="[val => !!val || 'Access key key is required']"
                />
                <q-input
                  v-model="form.s3.credentialsSecretRef.secretKeyKey"
                  label="Secret Key Key"
                  hint="Key within the Secret for the secret key"
                  outlined
                  :rules="[val => !!val || 'Secret key key is required']"
                />
              </template>
              <div class="text-subtitle2 text-grey q-mt-md">Staging Storage (optional)</div>
              <q-input
                v-model="form.s3.stagingSize"
                label="Staging PVC Size"
                hint="e.g. 1Gi, 10Gi — default: 1Gi"
                outlined
              />
              <q-input
                v-model="form.s3.stagingStorageClassName"
                label="Storage Class Name"
                hint="Override default from settings"
                outlined
              />
            </template>

            <!-- PVC Config -->
            <template v-if="jobType === 'pvc'">
              <div class="text-subtitle2 text-grey q-mt-md">PVC Configuration</div>
              <q-input
                v-model="form.pvc.claimName"
                label="Claim Name"
                outlined
                :rules="[val => !!val || 'Claim name is required']"
              />
              <q-input
                v-model="form.pvc.volumeSnapshotClassName"
                label="Volume Snapshot Class Name (optional)"
                outlined
              />
              <q-input
                v-model.number="form.pvc.snapshotRetention"
                label="Snapshot Retention Count (optional)"
                outlined
                type="number"
                :min="0"
              />
            </template>

            <!-- Submit -->
            <div class="row justify-end q-mt-lg">
              <q-btn
                label="Cancel"
                no-caps
                flat
                color="grey"
                :to="{ name: 'backupjob-detail', params: { namespace, name } }"
                class="q-mr-sm"
              />
              <q-btn
                type="submit"
                label="Save Changes"
                color="primary"
                no-caps
                unelevated
                :loading="submitting"
              />
            </div>
          </q-form>
        </q-card-section>
      </q-card>

      <!-- Error banner -->
      <q-banner v-if="submitError" class="bg-negative text-white q-mt-md" rounded>
        <template v-slot:avatar>
          <q-icon name="error" />
        </template>
        {{ submitError }}
      </q-banner>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import backupJobsApi, { type UpdateBackupJobRequest } from 'src/api/backupjobs';
import repositoriesApi, { type Repository } from 'src/api/repositories';

const route = useRoute();
const router = useRouter();
const namespace = route.params.namespace as string;
const name = route.params.name as string;

const loading = ref(true);
const loadError = ref('');
const jobType = ref('');

const repositories = ref<Repository[]>([]);
const loadingRepos = ref(true);

const s3CredentialMode = ref<'inline' | 'ref'>('ref');

const form = ref({
  schedule: '',
  repositoryRef: '',
  suspend: false,
  image: '',
  retention: {
    keepLast: undefined as number | undefined,
    keepDaily: undefined as number | undefined,
    keepWeekly: undefined as number | undefined,
    keepMonthly: undefined as number | undefined,
  },
  psql: {
    hostname: '',
    port: 5432 as number | undefined,
    username: '',
    database: '',
    password: '',
    stagingSize: '',
    stagingStorageClassName: '',
  },
  s3: {
    endpoint: '',
    bucket: '',
    accessKey: '',
    secretKey: '',
    credentialsSecretRef: {
      name: '',
      accessKeyKey: '',
      secretKeyKey: '',
    },
    stagingSize: '',
    stagingStorageClassName: '',
  },
  pvc: {
    claimName: '',
    volumeSnapshotClassName: '',
    snapshotRetention: undefined as number | undefined,
  },
});

const submitting = ref(false);
const submitError = ref('');

const repoOptions = computed(() =>
  repositories.value.map(r => ({
    label: `${r.name} (${r.namespace})`,
    value: `${r.namespace}/${r.name}`,
  }))
);

function buildRetention() {
  const r = form.value.retention;
  if (r.keepLast == null && r.keepDaily == null && r.keepWeekly == null && r.keepMonthly == null) {
    return undefined;
  }
  return {
    keepLast: r.keepLast || undefined,
    keepDaily: r.keepDaily || undefined,
    keepWeekly: r.keepWeekly || undefined,
    keepMonthly: r.keepMonthly || undefined,
  };
}

async function onSubmit() {
  submitting.value = true;
  submitError.value = '';
  try {
    const req: UpdateBackupJobRequest = {
      schedule: form.value.schedule,
      repositoryRef: form.value.repositoryRef,
      suspend: form.value.suspend,
      image: form.value.image || undefined,
      retention: buildRetention(),
    };

    if (jobType.value === 'psql') {
      req.psql = {
        hostname: form.value.psql.hostname,
        port: form.value.psql.port || undefined,
        username: form.value.psql.username,
        database: form.value.psql.database,
        password: form.value.psql.password || undefined,
      };
      if (form.value.psql.stagingSize) req.psql!.stagingSize = form.value.psql.stagingSize;
      if (form.value.psql.stagingStorageClassName) req.psql!.stagingStorageClassName = form.value.psql.stagingStorageClassName;
    } else if (jobType.value === 's3') {
      req.s3 = {
        endpoint: form.value.s3.endpoint,
        bucket: form.value.s3.bucket,
      };
      if (form.value.s3.stagingSize) req.s3.stagingSize = form.value.s3.stagingSize;
      if (form.value.s3.stagingStorageClassName) req.s3.stagingStorageClassName = form.value.s3.stagingStorageClassName;
      if (s3CredentialMode.value === 'inline') {
        req.s3.accessKey = form.value.s3.accessKey;
        req.s3.secretKey = form.value.s3.secretKey;
      } else {
        req.s3.credentialsSecretRef = {
          name: form.value.s3.credentialsSecretRef.name,
          accessKeyKey: form.value.s3.credentialsSecretRef.accessKeyKey,
          secretKeyKey: form.value.s3.credentialsSecretRef.secretKeyKey,
        };
      }
    } else if (jobType.value === 'pvc') {
      req.pvc = {
        claimName: form.value.pvc.claimName,
        volumeSnapshotClassName: form.value.pvc.volumeSnapshotClassName || undefined,
        snapshotRetention: form.value.pvc.snapshotRetention || undefined,
      };
    }

    await backupJobsApi.updateBackupJob(namespace, name, req);
    router.push({ name: 'backupjob-detail', params: { namespace, name } });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    submitError.value = `Failed to update backup job: ${msg}`;
    console.error('Failed to update backup job:', e);
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  try {
    const [job, repoList] = await Promise.all([
      backupJobsApi.getBackupJob(namespace, name),
      repositoriesApi.listRepositories(),
    ]);
    repositories.value = repoList;
    loadingRepos.value = false;

    jobType.value = job.type;
    form.value.schedule = job.schedule;
    form.value.repositoryRef = job.repositoryRef;
    form.value.suspend = job.suspend;
    form.value.image = job.image ?? '';

    if (job.retention) {
      form.value.retention.keepLast = job.retention.keepLast;
      form.value.retention.keepDaily = job.retention.keepDaily;
      form.value.retention.keepWeekly = job.retention.keepWeekly;
      form.value.retention.keepMonthly = job.retention.keepMonthly;
    }

    if (job.psql) {
      form.value.psql.hostname = job.psql.hostname;
      form.value.psql.port = job.psql.port ?? 5432;
      form.value.psql.username = job.psql.username;
      form.value.psql.database = job.psql.database;
      form.value.psql.stagingSize = job.psql.stagingSize ?? '';
      form.value.psql.stagingStorageClassName = job.psql.stagingStorageClassName ?? '';
    }

    if (job.s3) {
      form.value.s3.endpoint = job.s3.endpoint;
      form.value.s3.bucket = job.s3.bucket;
      form.value.s3.stagingSize = job.s3.stagingSize ?? '';
      form.value.s3.stagingStorageClassName = job.s3.stagingStorageClassName ?? '';
      form.value.s3.credentialsSecretRef.name = job.s3.credentialsSecretRef.name;
      form.value.s3.credentialsSecretRef.accessKeyKey = job.s3.credentialsSecretRef.accessKeyKey;
      form.value.s3.credentialsSecretRef.secretKeyKey = job.s3.credentialsSecretRef.secretKeyKey;
    }

    if (job.pvc) {
      form.value.pvc.claimName = job.pvc.claimName;
      form.value.pvc.volumeSnapshotClassName = job.pvc.volumeSnapshotClassName ?? '';
      form.value.pvc.snapshotRetention = job.pvc.snapshotRetention;
    }
  } catch (e) {
    loadError.value = 'Failed to load backup job.';
    console.error('Failed to load backup job:', e);
  } finally {
    loading.value = false;
    loadingRepos.value = false;
  }
});
</script>

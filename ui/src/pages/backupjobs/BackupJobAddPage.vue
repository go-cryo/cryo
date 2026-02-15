<template>
  <q-page padding>
    <div class="row items-center q-mb-lg">
      <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'backupjobs' }" class="q-mr-sm" />
      <div class="text-h5 text-weight-medium">Add Backup Job</div>
    </div>

    <q-card flat bordered class="cryo-card" style="max-width: 600px;">
      <q-card-section>
        <q-form @submit.prevent="onSubmit" class="q-gutter-md">
          <!-- Name -->
          <q-input
            v-model="form.name"
            label="Name"
            hint="Name of the backup job"
            outlined
            :rules="[val => !!val || 'Name is required', val => /^[a-z0-9][a-z0-9.-]*$/.test(val) || 'Must be a valid Kubernetes name']"
          />

          <!-- Namespace -->
          <q-input
            v-model="form.namespace"
            label="Namespace"
            hint="Kubernetes namespace (leave empty for default)"
            outlined
          />

          <!-- Type -->
          <q-select
            v-model="form.type"
            :options="typeOptions"
            label="Backup Type"
            outlined
            emit-value
            map-options
            :rules="[val => !!val || 'Type is required']"
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
          <template v-if="form.type === 'psql'">
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
              outlined
              type="password"
              :rules="[val => !!val || 'Password is required']"
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
          <template v-if="form.type === 's3'">
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
                { label: 'Create new secret', value: 'inline' },
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
          <template v-if="form.type === 'pvc'">
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
              :to="{ name: 'backupjobs' }"
              class="q-mr-sm"
            />
            <q-btn
              type="submit"
              label="Create Backup Job"
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
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import backupJobsApi, { type CreateBackupJobRequest } from 'src/api/backupjobs';
import repositoriesApi, { type Repository } from 'src/api/repositories';

const router = useRouter();

const repositories = ref<Repository[]>([]);
const loadingRepos = ref(true);

const form = ref({
  name: '',
  namespace: '',
  type: '' as '' | 'psql' | 's3' | 'pvc',
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

const s3CredentialMode = ref<'inline' | 'ref'>('inline');

const submitting = ref(false);
const submitError = ref('');

const typeOptions = [
  { label: 'PostgreSQL', value: 'psql' },
  { label: 'S3', value: 's3' },
  { label: 'PVC', value: 'pvc' },
];

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
  if (!form.value.type) return;
  submitting.value = true;
  submitError.value = '';
  try {
    const req: CreateBackupJobRequest = {
      name: form.value.name,
      namespace: form.value.namespace,
      type: form.value.type,
      schedule: form.value.schedule,
      repositoryRef: form.value.repositoryRef,
      suspend: form.value.suspend || undefined,
      image: form.value.image || undefined,
      retention: buildRetention(),
    };

    if (form.value.type === 'psql') {
      req.psql = {
        hostname: form.value.psql.hostname,
        port: form.value.psql.port || undefined,
        username: form.value.psql.username,
        database: form.value.psql.database,
        password: form.value.psql.password,
      };
      if (form.value.psql.stagingSize) req.psql!.stagingSize = form.value.psql.stagingSize;
      if (form.value.psql.stagingStorageClassName) req.psql!.stagingStorageClassName = form.value.psql.stagingStorageClassName;
    } else if (form.value.type === 's3') {
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
    } else if (form.value.type === 'pvc') {
      req.pvc = {
        claimName: form.value.pvc.claimName,
        volumeSnapshotClassName: form.value.pvc.volumeSnapshotClassName || undefined,
        snapshotRetention: form.value.pvc.snapshotRetention || undefined,
      };
    }

    const created = await backupJobsApi.createBackupJob(req);
    router.push({ name: 'backupjob-detail', params: { namespace: created.namespace, name: created.name } });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    submitError.value = `Failed to create backup job: ${msg}`;
    console.error('Failed to create backup job:', e);
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  try {
    repositories.value = await repositoriesApi.listRepositories();
  } catch (e) {
    console.error('Failed to fetch repositories:', e);
  } finally {
    loadingRepos.value = false;
  }
});
</script>

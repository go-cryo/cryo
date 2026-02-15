<template>
  <q-page padding>
    <div class="row items-center q-mb-lg">
      <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'hosts' }" class="q-mr-sm" />
      <div class="text-h5 text-weight-medium">Add Host</div>
    </div>

    <q-card flat bordered class="cryo-card" style="max-width: 600px;">
      <q-card-section>
        <q-form @submit.prevent="onSubmit" class="q-gutter-md">
          <!-- Name -->
          <q-input
            v-model="form.name"
            label="Secret Name"
            hint="Name of the Kubernetes Secret to create"
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

          <!-- Host type selector -->
          <q-select
            v-model="hostType"
            :options="hostTypeOptions"
            label="Host Type"
            outlined
            emit-value
            map-options
          />

          <!-- Base URL -->
          <q-input
            v-model="form.baseUrl"
            label="Base URL"
            :hint="baseUrlHint"
            :placeholder="baseUrlPlaceholder"
            outlined
            :rules="[val => !!val || 'Base URL is required']"
          />

          <!-- S3 credentials (shown when type is s3) -->
          <template v-if="hostType === 's3'">
            <q-separator class="q-my-sm" />
            <div class="text-subtitle2 text-grey">S3 Credentials</div>

            <q-input
              v-model="form.awsAccessKeyId"
              label="AWS Access Key ID"
              outlined
            />
            <q-input
              v-model="form.awsSecretAccessKey"
              label="AWS Secret Access Key"
              outlined
              :type="showAwsSecret ? 'text' : 'password'"
            >
              <template v-slot:append>
                <q-icon
                  :name="showAwsSecret ? 'visibility_off' : 'visibility'"
                  class="cursor-pointer"
                  @click="showAwsSecret = !showAwsSecret"
                />
              </template>
            </q-input>
            <q-input
              v-model="form.awsDefaultRegion"
              label="AWS Region"
              placeholder="eu-central-1"
              outlined
            />
          </template>

          <!-- Submit -->
          <div class="row justify-end q-mt-lg">
            <q-btn
              label="Cancel"
              no-caps
              flat
              color="grey"
              :to="{ name: 'hosts' }"
              class="q-mr-sm"
            />
            <q-btn
              type="submit"
              label="Create Host"
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
import { ref, computed } from 'vue';
import { useRouter } from 'vue-router';
import hostsApi, { type CreateHostRequest } from 'src/api/hosts';

const router = useRouter();

const hostType = ref<'s3' | 'local' | 'sftp' | 'rest'>('s3');
const hostTypeOptions = [
  { label: 'S3', value: 's3' },
  { label: 'Local', value: 'local' },
  { label: 'SFTP', value: 'sftp' },
  { label: 'REST Server', value: 'rest' },
];

const form = ref<CreateHostRequest>({
  name: '',
  namespace: '',
  baseUrl: '',
  awsAccessKeyId: '',
  awsSecretAccessKey: '',
  awsDefaultRegion: '',
});

const showAwsSecret = ref(false);
const submitting = ref(false);
const submitError = ref('');

const baseUrlHint = computed(() => {
  switch (hostType.value) {
    case 's3': return 'e.g. s3:https://s3.amazonaws.com';
    case 'sftp': return 'e.g. sftp:user@host:';
    case 'rest': return 'e.g. rest:https://host:8000';
    case 'local': return 'e.g. /srv/restic';
    default: return '';
  }
});

const baseUrlPlaceholder = computed(() => {
  switch (hostType.value) {
    case 's3': return 's3:https://s3.amazonaws.com';
    case 'sftp': return 'sftp:user@host:';
    case 'rest': return 'rest:https://host:8000';
    case 'local': return '/srv/restic';
    default: return '';
  }
});

async function onSubmit() {
  submitting.value = true;
  submitError.value = '';
  try {
    const created = await hostsApi.createHost(form.value);
    router.push({ name: 'host-detail', params: { namespace: created.namespace, name: created.name } });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    submitError.value = `Failed to create host: ${msg}`;
    console.error('Failed to create host:', e);
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <q-page padding>
    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading repository...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="loadError" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ loadError }}</div>
      <q-btn class="q-mt-md" color="primary" label="Back" no-caps outline :to="{ name: 'repositories' }" />
    </div>

    <template v-else>
      <div class="row items-center q-mb-lg">
        <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'repository-detail', params: { namespace, name } }" class="q-mr-sm" />
        <div class="text-h5 text-weight-medium">Edit Repository</div>
      </div>

      <q-card flat bordered class="cryo-card" style="max-width: 600px;">
        <q-card-section>
          <q-form @submit.prevent="onSubmit" class="q-gutter-md">
            <!-- Name (read-only) -->
            <q-input
              :model-value="name"
              label="Secret Name"
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

            <!-- Host selection -->
            <q-select
              v-model="selectedHost"
              :options="hostOptions"
              label="Host"
              outlined
              emit-value
              map-options
              :loading="loadingHosts"
              :rules="[val => !!val || 'Host is required']"
            >
              <template v-slot:no-option>
                <q-item>
                  <q-item-section class="text-grey">
                    No hosts available.
                    <router-link :to="{ name: 'host-add' }" class="text-primary">Create one first.</router-link>
                  </q-item-section>
                </q-item>
              </template>
            </q-select>

            <!-- Path -->
            <q-input
              v-model="form.path"
              label="Repository Path"
              :hint="pathHint"
              :placeholder="pathPlaceholder"
              outlined
              :rules="[val => !!val || 'Path is required']"
            />

            <!-- URL Preview -->
            <div v-if="previewUrl" class="text-caption text-grey q-px-sm">
              Resolved URL: <span class="text-mono">{{ previewUrl }}</span>
            </div>

            <!-- Restic Password -->
            <q-input
              v-model="form.resticPassword"
              label="Restic Password"
              hint="Leave empty to keep the current password"
              outlined
              :type="showPassword ? 'text' : 'password'"
            >
              <template v-slot:append>
                <q-icon
                  :name="showPassword ? 'visibility_off' : 'visibility'"
                  class="cursor-pointer"
                  @click="showPassword = !showPassword"
                />
              </template>
            </q-input>

            <!-- Submit -->
            <div class="row justify-end q-mt-lg">
              <q-btn
                label="Cancel"
                no-caps
                flat
                color="grey"
                :to="{ name: 'repository-detail', params: { namespace, name } }"
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
import repositoriesApi from 'src/api/repositories';
import hostsApi, { type RepositoryHost } from 'src/api/hosts';

const route = useRoute();
const router = useRouter();
const namespace = route.params.namespace as string;
const name = route.params.name as string;

const loading = ref(true);
const loadError = ref('');

const hosts = ref<RepositoryHost[]>([]);
const loadingHosts = ref(true);
const selectedHost = ref<string | null>(null);

const form = ref({
  path: '',
  resticPassword: '',
});

const showPassword = ref(false);
const submitting = ref(false);
const submitError = ref('');

const hostOptions = computed(() =>
  hosts.value.map(h => ({
    label: `${h.name} (${h.type})`,
    value: `${h.namespace}/${h.name}`,
  }))
);

const currentHost = computed(() => {
  if (!selectedHost.value) return null;
  const [ns, n] = selectedHost.value.split('/');
  return hosts.value.find(h => h.namespace === ns && h.name === n) ?? null;
});

const pathHint = computed(() => {
  if (!currentHost.value) return 'Select a host first';
  switch (currentHost.value.type) {
    case 's3': return 'e.g. my-bucket/daily-backups';
    case 'sftp': return 'e.g. /path/to/repo';
    case 'rest': return 'e.g. /path/to/repo';
    case 'local': return 'e.g. my-backups';
    default: return '';
  }
});

const pathPlaceholder = computed(() => {
  if (!currentHost.value) return '';
  switch (currentHost.value.type) {
    case 's3': return 'my-bucket/daily-backups';
    case 'sftp': return '/path/to/repo';
    case 'rest': return '/path/to/repo';
    case 'local': return 'my-backups';
    default: return '';
  }
});

const previewUrl = computed(() => {
  if (!currentHost.value || !form.value.path) return '';
  const base = currentHost.value.baseUrl.replace(/\/+$/, '');
  const path = form.value.path.replace(/^\/+/, '');
  return `${base}/${path}`;
});

async function onSubmit() {
  if (!selectedHost.value) return;
  submitting.value = true;
  submitError.value = '';
  try {
    await repositoriesApi.updateRepository(namespace, name, {
      hostRef: selectedHost.value,
      path: form.value.path,
      resticPassword: form.value.resticPassword,
    });
    router.push({ name: 'repository-detail', params: { namespace, name } });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    submitError.value = `Failed to update repository: ${msg}`;
    console.error('Failed to update repository:', e);
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  try {
    const [repo, hostList] = await Promise.all([
      repositoriesApi.getRepository(namespace, name),
      hostsApi.listHosts(),
    ]);
    hosts.value = hostList;
    loadingHosts.value = false;

    if (repo.hostRef) {
      selectedHost.value = repo.hostRef;
    }
    if (repo.path) {
      form.value.path = repo.path;
    }
  } catch (e) {
    loadError.value = 'Failed to load repository.';
    console.error('Failed to load repository:', e);
  } finally {
    loading.value = false;
    loadingHosts.value = false;
  }
});
</script>

<style lang="scss" scoped>
.text-mono {
  font-family: 'Roboto Mono', monospace;
  font-size: 0.85em;
}
</style>

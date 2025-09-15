package com.productscience

import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

@Tag("exclude")
class ApiConfigTests {

    private val mockYamlContent = """
api:
    admin_server_port: 9200
    ml_grpc_callback_address: ""
    ml_grpc_server_port: 9300
    ml_server_port: 9100
    poc_callback_url: http://join1-api:9100
    port: 8080
    public_server_port: 9000
    public_url: http://join1-api:9000
    test_mode: true
bandwidth_params:
    estimated_limits_per_block_kb: 43008
    kb_per_input_token: 0.0023
    kb_per_output_token: 0.64
chain_node:
    KeyringPassword: join100000
    account_public_key: AqGcryRTgpMC+VydQUZJwhV58SHcM60EeaCr4x3pTx00
    is_genesis: false
    keyring_backend: file
    keyring_dir: /root/.inference
    seed_api_url: http://genesis-api:9000
    signer_key_name: join1-WARM
    url: http://join1-node:26657
current_height: 163
current_node_version: ""
current_seed:
    epoch_index: 10
    seed: 517365417585945315
    signature: 9ba03b1bde76ce57dc8b439e1dd883e3f6b2bf97f07b8fc8691e32fb5d323bf31dd0167f27488fe9749f978d36ad700bac1d7a43b4e4406cec33e9ac5d941d2d
merged_node_config: true
ml_node_key_config:
    worker_private: ""
    worker_public: ""
nats:
    host: ""
    port: 0
node_versions:
    versions: []
nodes:
    - hardware: []
      host: join1-mock-server
      id: wiremock2
      inference_port: 8080
      inference_segment: ""
      max_concurrent: 1000
      models:
        Qwen/Qwen2.5-7B-Instruct:
            Args: []
      poc_port: 8080
      poc_segment: ""
      version: ""
previous_seed:
    epoch_index: 9
    seed: 3888597983176793143
    signature: b79d968ed06e4c394d3ecedb3e8a0195db891ddce4648514cdcffde41d6f990129b5553bf3a9382f97983918be25547185527aa953eb33c2a04ab9d1f416878e
upcoming_seed:
    epoch_index: 0
    seed: 0
    signature: ""
upgrade_plan:
    binaries: {}
    height: 0
    name: ""
validation_params:
    expiration_blocks: 7
    timestamp_advance: 30
    timestamp_expiration: 60
    """.trimIndent()

    class MockCliExecutor(private val yamlContent: String) : CliExecutor {
        override fun exec(args: List<String>, stdin: String?): List<String> {
            if (args == listOf("cat", "/root/.dapi/api-config.yaml")) {
                return yamlContent.chunked(500)
            }
            throw UnsupportedOperationException("Mock executor only supports cat command")
        }

        override fun createContainer(doNotStartChain: Boolean) {
            // Not implemented for test
        }

        override fun kill() {
            // Not implemented for test
        }
    }

    @Test
    fun `getConfig should parse YAML configuration correctly`() {
        val mockExecutor = MockCliExecutor(mockYamlContent)
        val applicationConfig = ApplicationConfig(
            appName = "test",
            chainId = "test-chain",
            nodeImageName = "test-node",
            genesisNodeImage = "test-genesis",
            apiImageName = "test-api",
            mockImageName = "test-mock",
            denom = "ngonka",
            stateDirName = "test-state"
        )
        val logOutput = LogOutput("test", "console")
        val applicationAPI = ApplicationAPI(
            urls = mapOf("public" to "http://test:9000"),
            config = applicationConfig,
            logOutput = logOutput,
            executor = mockExecutor
        )

        val config = applicationAPI.getConfig()

        assertNotNull(config)

        // Test API settings
        assertEquals(9200, config.api.adminServerPort)
        assertEquals("", config.api.mlGrpcCallbackAddress)
        assertEquals(9300, config.api.mlGrpcServerPort)
        assertEquals(9100, config.api.mlServerPort)
        assertEquals("http://join1-api:9100", config.api.pocCallbackUrl)
        assertEquals(8080, config.api.port)
        assertEquals(9000, config.api.publicServerPort)
        assertEquals("http://join1-api:9000", config.api.publicUrl)
        assertEquals(true, config.api.testMode)

        // Test bandwidth params
        assertEquals(43008L, config.bandwidthParams.estimatedLimitsPerBlockKb)
        assertEquals(0.0023, config.bandwidthParams.kbPerInputToken, 0.0001)
        assertEquals(0.64, config.bandwidthParams.kbPerOutputToken, 0.0001)

        // Test chain node
        assertEquals("join100000", config.chainNode.keyringPassword)
        assertEquals("AqGcryRTgpMC+VydQUZJwhV58SHcM60EeaCr4x3pTx00", config.chainNode.accountPublicKey)
        assertEquals(false, config.chainNode.isGenesis)
        assertEquals("file", config.chainNode.keyringBackend)
        assertEquals("/root/.inference", config.chainNode.keyringDir)
        assertEquals("http://genesis-api:9000", config.chainNode.seedApiUrl)
        assertEquals("join1-WARM", config.chainNode.signerKeyName)
        assertEquals("http://join1-node:26657", config.chainNode.url)

        // Test current_height and other top-level fields
        assertEquals(163L, config.currentHeight)
        assertEquals("", config.currentNodeVersion)
        assertEquals(true, config.mergedNodeConfig)

        // Test current seed
        assertEquals(10L, config.currentSeed.epochIndex)
        assertEquals(517365417585945315L, config.currentSeed.seed)
        assertEquals(
            "9ba03b1bde76ce57dc8b439e1dd883e3f6b2bf97f07b8fc8691e32fb5d323bf31dd0167f27488fe9749f978d36ad700bac1d7a43b4e4406cec33e9ac5d941d2d",
            config.currentSeed.signature
        )

        // Test ml_node_key_config
        assertEquals("", config.mlNodeKeyConfig.workerPrivate)
        assertEquals("", config.mlNodeKeyConfig.workerPublic)

        // Test nats
        assertEquals("", config.nats.host)
        assertEquals(0, config.nats.port)

        // Test node_versions
        assertEquals(emptyList<String>(), config.nodeVersions.versions)

        // Test nodes
        assertEquals(1, config.nodes.size)
        val node = config.nodes[0]
        assertEquals(emptyList(), node.hardware)
        assertEquals("join1-mock-server", node.host)
        assertEquals("wiremock2", node.id)
        assertEquals(8080, node.inferencePort)
        assertEquals("", node.inferenceSegment)
        assertEquals(1000, node.maxConcurrent)
        assertEquals(1, node.models.size)
        assertEquals(emptyList<String>(), node.models["Qwen/Qwen2.5-7B-Instruct"]?.args)
        assertEquals(8080, node.pocPort)
        assertEquals("", node.pocSegment)
        assertEquals("", node.version)

        // Test previous seed
        assertEquals(9L, config.previousSeed.epochIndex)
        assertEquals(3888597983176793143L, config.previousSeed.seed)
        assertEquals(
            "b79d968ed06e4c394d3ecedb3e8a0195db891ddce4648514cdcffde41d6f990129b5553bf3a9382f97983918be25547185527aa953eb33c2a04ab9d1f416878e",
            config.previousSeed.signature
        )

        // Test upcoming seed
        assertEquals(0L, config.upcomingSeed.epochIndex)
        assertEquals(0L, config.upcomingSeed.seed)
        assertEquals("", config.upcomingSeed.signature)

        // Test upgrade plan
        assertEquals(emptyMap<String, String>(), config.upgradePlan.binaries)
        assertEquals(0L, config.upgradePlan.height)
        assertEquals("", config.upgradePlan.name)

        // Test validation params
        assertEquals(7, config.validationParams.expirationBlocks)
        assertEquals(30, config.validationParams.timestampAdvance)
        assertEquals(60, config.validationParams.timestampExpiration)
    }

    @Test
    fun `getConfig should handle executor errors gracefully`() {
        val failingExecutor = object : CliExecutor {
            override fun exec(args: List<String>, stdin: String?): List<String> {
                throw RuntimeException("Command execution failed")
            }

            override fun createContainer(doNotStartChain: Boolean) {}
            override fun kill() {}
        }

        val applicationConfig = ApplicationConfig(
            appName = "test",
            chainId = "test-chain",
            nodeImageName = "test-node",
            genesisNodeImage = "test-genesis",
            apiImageName = "test-api",
            mockImageName = "test-mock",
            denom = "ngonka",
            stateDirName = "test-state"
        )
        val logOutput = LogOutput("test", "console")
        val applicationAPI = ApplicationAPI(
            urls = mapOf("public" to "http://test:9000"),
            config = applicationConfig,
            logOutput = logOutput,
            executor = failingExecutor
        )

        assertThrows<RuntimeException> {
            applicationAPI.getConfig()
        }
    }
}
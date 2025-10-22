package com.productscience.data

data class StartTrainingDto(
    val hardwareResources: List<HardwareResourcesDto>,
    val config: TrainingConfigDto
)

data class HardwareResourcesDto(
    val type: String,
    val count: UInt
)

data class TrainingConfigDto(
    val datasets: TrainingDatasetsDto,
    val numUocEstimationSteps: UInt
)

data class TrainingDatasetsDto(
    val train: String,
    val test: String
)

data class LockTrainingNodesDto(
    val trainingTaskId: ULong,
    val nodeIds: List<String>
)



data class MsgAssignTrainingTask(
    override val type: String = "/inference.inference.MsgAssignTrainingTask",
    val creator: String = "",
    val taskId: Long = 0L,
    val assignees: List<TrainingTaskAssignee>,
) : TxMessage

data class TrainingTaskAssignee(
    val participant: String = "",
    val nodeIds: List<String> = listOf()
)

data class MsgClaimTrainingTaskForAssignment(
    override val type: String = "/inference.inference.MsgClaimTrainingTaskForAssignment",
    val creator: String = "",
    val taskId: Long = 0L,
) : TxMessage

data class MsgCreateDummyTrainingTask(
    override val type: String = "/inference.inference.MsgCreateDummyTrainingTask",
    val creator: String = "",
    val task: TrainingTask
) : TxMessage

data class TrainingTask(
    val id: Long = 0L,
    val requestedBy: String = "",
    val createdAtBlockHeight: Long = 0L,
    val assigner: String = "",
    val claimedByAssignerAtBlockHeight: Long = 0L,
    val assignedAtBlockHeight: Long = 0L,
    val finishedAtBlockHeight: Long = 0L,
    val hardwareResources: List<TrainingHardwareResources> = listOf(),
    val config: TrainingConfig = TrainingConfig(),
    val assignees: List<TrainingTaskAssignee> = listOf(),
    val epoch: EpochInfo = EpochInfo()
)

data class TrainingHardwareResources(
    val type: String = "",
    val count: Long = 0L
)

data class TrainingConfig(
    val datasets: TrainingDatasets = TrainingDatasets(),
    val numUocEstimationSteps: Long = 0L
)

data class TrainingDatasets(
    val train: String = "",
    val test: String = ""
)

data class EpochInfo(
    val lastEpoch: Int = 0,
    val lastEpochBlockHeight: Long = 0L,
    val lastEpochTimestamp: Long = 0L,
    val lastEpochIsFinished: Boolean = false
)

data class MsgCreateTrainingTask(
    override val type: String = "/inference.inference.MsgCreateTrainingTask",
    val creator: String = "",
    val hardwareResources: List<TrainingHardwareResources>,
    val config: TrainingConfig,
) : TxMessage

data class MsgJoinTraining(
    override val type: String = "/inference.inference.MsgJoinTraining",
    val creator: String = "",
    val req: JoinTrainingRequest,
) : TxMessage

data class JoinTrainingRequest(
    val nodeId: String = "",
    val runId: Long = 0,
    val outerStep: Int = 0,
)

data class MsgJoinTrainingStatus(
    override val type: String = "/inference.inference.MsgJoinTrainingStatus",
    val creator: String = "",
    val req: JoinTrainingRequest
) : TxMessage

data class MsgSetBarrier(
    override val type: String = "/inference.inference.MsgSetBarrier",
    val creator: String = "",
    val req: SetBarrierRequest,
) : TxMessage

data class SetBarrierRequest(
    val barrierId: String = "",
    val nodeId: String = "",
    val runId: Long = 0,
    val outerStep: Int = 0,
)

data class MsgSubmitTrainingKvRecord(
    override val type: String = "/inference.inference.MsgSubmitTrainingKvRecord",
    val creator: String = "",
    val taskId: Long = 0L,
    val participant: String = "",
    val key: String = "",
    val value: String = "",
) : TxMessage

data class MsgTrainingHeartbeat(
    override val type: String = "/inference.inference.MsgTrainingHeartbeat",
    val creator: String = "",
    val req: HeartbeatRequest,
) : TxMessage

data class HeartbeatRequest(
    val nodeId: String = "",
    val runId: Long = 0,
    val localRank: Int = 0,
    val timestamp: Double = 0.0,
    val innerStep: Int = 0,
    val outerStep: Int = 0,
    val epoch: Int = 0,
)
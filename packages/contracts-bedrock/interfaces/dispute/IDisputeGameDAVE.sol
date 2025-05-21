// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { IInitializable } from "interfaces/dispute/IInitializable.sol";
import { Timestamp, GameStatus, GameType, Claim, Hash } from "src/dispute/lib/Types.sol";

/**
 * @title ITournamentGame Interface
 * @notice Interface for tournament-based dispute games in the Cartesi DAVE fraud proofs system
 * @dev Extends the IDisputeGame interface with tournament-specific functionality
 */
interface IDisputeGameDAVE is IInitializable {

    /**
     * @notice Emitted when the dispute game is resolved
     * @param status The status of the game after resolution
     */
    event Resolved(GameStatus indexed status);

    /**
     * @notice Returns the timestamp when the dispute game was created
     * @return The timestamp when the dispute game was created
     */
    function createdAt() external view returns (Timestamp);

    /**
     * @notice Returns the timestamp when the dispute game was resolved
     * @return The timestamp when the dispute game was resolved
     */
    function resolvedAt() external view returns (Timestamp);

    /**
     * @notice Returns the current status of the dispute game
     * @return The current status of the dispute game
     */
    function status() external view returns (GameStatus);

    /**
     * @notice Returns the type of the dispute game
     * @return The type of the dispute game
     */
    function gameType() external pure returns (GameType gameType_);

    /**
     * @notice Returns the address that created the dispute game
     * @return The address that created the dispute game
     */
    function gameCreator() external pure returns (address creator_);

    /**
     * @notice Returns the root claim of the dispute game
     * @return The root claim of the dispute game
     */
    function rootClaim() external pure returns (Claim rootClaim_);

    /**
     * @notice Returns the L1 head hash at the time the dispute game was created
     * @return The L1 head hash at the time the dispute game was created
     */
    function l1Head() external pure returns (Hash l1Head_);
    function l2SequenceNumber() external pure returns (uint256 l2SequenceNumber_);

    /**
     * @notice Returns extra data supplied to the dispute game
     * @return Extra data supplied to the dispute game
     */
    function extraData() external pure returns (bytes memory extraData_);

    /**
     * @notice Returns the game type, root claim, and extra data
     * @return The game type, root claim, and extra data
     */
    function gameData() external view returns (GameType gameType_, Claim rootClaim_, bytes memory extraData_);

    /**
     * @notice Resolves the dispute game
     * @return The status of the game after resolution
     */
    function resolve() external returns (GameStatus status_);
    function wasRespectedGameTypeWhenCreated() external view returns (bool);

}

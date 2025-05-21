// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { IDisputeGame } from "./IDisputeGame.sol";

/**
 * @title ITournament
 * @notice Interface for the Tournament contract
 */
interface ITournament {
    /**
     * @notice Struct representing a node in the tournament tree
     */
    struct Node {
        bytes32 claim;        // The claim made by the participant
        address claimant;     // Address of the participant who made the claim
        uint64 timestamp;     // Timestamp when the claim was made
        bool challenged;      // Whether this claim has been challenged
    }

    /**
     * @notice Struct representing a match between two claims
     */
    struct Match {
        uint256 nodeIndex1;   // Index of the first node (defender)
        uint256 nodeIndex2;   // Index of the second node (challenger)
        uint64 deadline;      // Deadline for resolving the match
        uint256 winner;       // Index of the winning node (0 if not resolved)
        bytes32 evidence;     // Evidence hash for the match
    }

    /**
     * @notice Event emitted when a new claim is added to the tournament
     */
    event ClaimAdded(uint256 indexed nodeIndex, bytes32 indexed claim, address indexed claimant);

    /**
     * @notice Event emitted when a match is created
     */
    event MatchCreated(uint256 indexed matchIndex, uint256 indexed nodeIndex1, uint256 indexed nodeIndex2);

    /**
     * @notice Event emitted when a match is resolved
     */
    event MatchResolved(uint256 indexed matchIndex, uint256 indexed winner);

    /**
     * @notice Event emitted when the tournament is resolved
     */
    event TournamentResolved(IDisputeGame.GameStatus status, bytes32 winningClaim);

    /**
     * @notice Join the tournament with a counter-claim
     * @param claim The claim to submit
     * @param parentClaimIndex The index of the parent claim being challenged
     * @return The index of the newly created node
     */
    function joinTournament(bytes32 claim, uint256 parentClaimIndex) external payable returns (uint256);

    /**
     * @notice Submit evidence for a match
     * @param matchIndex The index of the match
     * @param evidence The evidence hash
     */
    function submitEvidence(uint256 matchIndex, bytes32 evidence) external;

    /**
     * @notice Resolve a match in the tournament
     * @param matchIndex The index of the match to resolve
     */
    function resolveMatch(uint256 matchIndex) external;

    /**
     * @notice Get the result of the tournament
     * @return The status of the game and the winning claim
     */
    function result() external view returns (IDisputeGame.GameStatus, bytes32);

    /**
     * @notice Get the number of nodes in the tournament
     * @return The number of nodes
     */
    function getNodeCount() external view returns (uint256);

    /**
     * @notice Get a node by index
     * @param index The index of the node
     * @return The node
     */
    function getNode(uint256 index) external view returns (Node memory);

    /**
     * @notice Get the number of matches in the tournament
     * @return The number of matches
     */
    function getMatchCount() external view returns (uint256);

    /**
     * @notice Get a match by index
     * @param index The index of the match
     * @return The match
     */
    function getMatch(uint256 index) external view returns (Match memory);
}